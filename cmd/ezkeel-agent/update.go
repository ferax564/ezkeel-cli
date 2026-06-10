package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ferax564/ezkeel-cli/internal/version"
	"github.com/ferax564/ezkeel-cli/pkg/agent"
)

// releaseAssetURL is the GitHub asset pattern self-update falls back to
// when neither the request nor EZKEEL_AGENT_DOWNLOAD_URL override it.
// Mirrors pkg/bootstrap.DefaultAgentURL ({ARCH} substituted at runtime).
const (
	latestAssetURL = "https://github.com/ferax564/ezkeel-cli/releases/latest/download/ezkeel-agent-linux-{ARCH}"
	pinnedAssetURL = "https://github.com/ferax564/ezkeel-cli/releases/download/%s/ezkeel-agent-linux-{ARCH}"
)

// updateOptions carries the environment-dependent pieces of self-update
// so tests can point it at an httptest server and a temp destination.
type updateOptions struct {
	// destPath is the binary to replace. Empty means the running
	// executable (os.Executable, symlinks resolved).
	destPath string
	// httpClient performs the downloads. Nil means a 60s-timeout client.
	httpClient *http.Client
	// arch substitutes the {ARCH} URL placeholder. Empty means
	// runtime.GOARCH.
	arch string
}

func defaultUpdateOptions() updateOptions {
	return updateOptions{}
}

// handleUpdate replaces the agent binary with a release build:
// download to a temp file next to the target, verify a SHA256 checksum
// (explicit from the request, or looked up in the release's SHA256SUMS),
// sanity-check the new binary executes, then atomically rename it over
// the old one. The agent is invoked per-request over SSH rather than
// running as a daemon, so the rename is safe — the next invocation picks
// up the new binary while the current process keeps executing the old
// (already-mapped) image.
//
// Updates without any obtainable checksum are refused: this binary runs
// as root on customer machines, so "trust whatever the server returned"
// is not an acceptable fallback.
func handleUpdate(r runner, req *agent.UpdateRequest, opts updateOptions) *agent.Response {
	dest := opts.destPath
	if dest == "" {
		exe, err := os.Executable()
		if err != nil {
			return respErr("cannot locate running binary: " + err.Error())
		}
		if exe, err = filepath.EvalSymlinks(exe); err != nil {
			return respErr("cannot resolve binary path: " + err.Error())
		}
		dest = exe
	}
	httpClient := opts.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	arch := opts.arch
	if arch == "" {
		arch = runtime.GOARCH
	}

	downloadURL, err := resolveUpdateURL(req, arch)
	if err != nil {
		return respErr(err.Error())
	}

	binData, err := fetchURL(httpClient, downloadURL)
	if err != nil {
		return respErr(fmt.Sprintf("download %s: %v", downloadURL, err))
	}

	expected := strings.ToLower(strings.TrimSpace(req.SHA256))
	if expected == "" {
		expected, err = lookupReleaseChecksum(httpClient, downloadURL)
		if err != nil {
			return respErr(fmt.Sprintf("no checksum available for %s — refusing unverified update: %v", downloadURL, err))
		}
	}
	sum := sha256.Sum256(binData)
	actual := hex.EncodeToString(sum[:])
	if actual != expected {
		return respErr(fmt.Sprintf("checksum mismatch for %s: expected %s, got %s", downloadURL, expected, actual))
	}

	// Write to a temp file in the destination directory so the final
	// rename stays on one filesystem (and therefore atomic).
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".ezkeel-agent-update-*")
	if err != nil {
		return respErr("create temp file: " + err.Error())
	}
	tmpPath := tmp.Name()
	cleanup := func() { os.Remove(tmpPath) } //nolint:errcheck
	if _, err := tmp.Write(binData); err != nil {
		tmp.Close() //nolint:errcheck
		cleanup()
		return respErr("write temp file: " + err.Error())
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return respErr("close temp file: " + err.Error())
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		cleanup()
		return respErr("chmod temp file: " + err.Error())
	}

	// Sanity-check the new binary actually executes BEFORE the swap —
	// catches truncated downloads and wrong-arch binaries while the old
	// binary is still in place.
	verifyOut, err := r.CombinedOutput(tmpPath, "--version")
	if err != nil {
		cleanup()
		return respErr(fmt.Sprintf("new binary failed to execute (--version): %v: %s", err, strings.TrimSpace(string(verifyOut))))
	}

	if err := os.Rename(tmpPath, dest); err != nil {
		cleanup()
		return respErr(fmt.Sprintf("replace %s: %v", dest, err))
	}

	return respOK(fmt.Sprintf(
		"updated %s: v%s -> %s (sha256 %s); takes effect on the next agent invocation",
		dest, version.Version, strings.TrimSpace(string(verifyOut)), actual,
	))
}

// resolveUpdateURL picks the download URL in precedence order: explicit
// request URL, the host's EZKEEL_AGENT_DOWNLOAD_URL, then the GitHub
// release pattern (pinned to req.Version when given, latest otherwise).
// The {ARCH} placeholder is substituted in every case.
func resolveUpdateURL(req *agent.UpdateRequest, arch string) (string, error) {
	u := req.DownloadURL
	if u == "" {
		u = os.Getenv("EZKEEL_AGENT_DOWNLOAD_URL")
	}
	if u == "" {
		if req.Version != "" {
			u = fmt.Sprintf(pinnedAssetURL, req.Version)
		} else {
			u = latestAssetURL
		}
	}
	u = strings.ReplaceAll(u, "{ARCH}", arch)
	parsed, err := url.Parse(u)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("invalid download URL %q", u)
	}
	return u, nil
}

// lookupReleaseChecksum fetches the SHA256SUMS file that release builds
// publish next to each binary and returns the hex digest recorded for
// binURL's asset name.
func lookupReleaseChecksum(c *http.Client, binURL string) (string, error) {
	parsed, err := url.Parse(binURL)
	if err != nil {
		return "", err
	}
	asset := path.Base(parsed.Path)
	parsed.Path = path.Dir(parsed.Path) + "/SHA256SUMS"
	parsed.RawQuery = ""

	sums, err := fetchURL(c, parsed.String())
	if err != nil {
		return "", fmt.Errorf("fetch SHA256SUMS: %w", err)
	}

	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("asset %s not listed in SHA256SUMS", asset)
}

// fetchURL GETs u and returns the body, treating non-2xx as an error.
func fetchURL(c *http.Client, u string) ([]byte, error) {
	resp, err := c.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 512<<20))
}
