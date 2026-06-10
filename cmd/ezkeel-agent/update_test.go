package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ferax564/ezkeel-cli/pkg/agent"
)

// fakeRelease serves a binary and its SHA256SUMS the way a GitHub
// release does, and returns the server plus the asset URL.
func fakeRelease(t *testing.T, binary []byte, withSums bool) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(binary)
	sums := fmt.Sprintf("%s  ezkeel-agent-linux-amd64\n", hex.EncodeToString(sum[:]))

	mux := http.NewServeMux()
	mux.HandleFunc("/release/ezkeel-agent-linux-amd64", func(w http.ResponseWriter, r *http.Request) {
		w.Write(binary) //nolint:errcheck
	})
	if withSums {
		mux.HandleFunc("/release/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(sums)) //nolint:errcheck
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// updateDest creates a fake installed binary and returns its path.
func updateDest(t *testing.T) string {
	t.Helper()
	dest := filepath.Join(t.TempDir(), "ezkeel-agent")
	if err := os.WriteFile(dest, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dest
}

func TestHandleUpdate_VerifiedAgainstSHA256SUMS(t *testing.T) {
	binary := []byte("new-binary-contents")
	srv := fakeRelease(t, binary, true)
	dest := updateDest(t)

	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			return []byte("ezkeel-agent v9.9.9"), nil // the --version sanity check
		},
	}
	resp := handleUpdate(r, &agent.UpdateRequest{
		DownloadURL: srv.URL + "/release/ezkeel-agent-linux-{ARCH}",
	}, updateOptions{destPath: dest, httpClient: srv.Client(), arch: "amd64"})

	if !resp.OK {
		t.Fatalf("update failed: %s", resp.Error)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(binary) {
		t.Errorf("binary was not replaced: %q", got)
	}
	info, _ := os.Stat(dest)
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v, want 0755", info.Mode().Perm())
	}
	if !strings.Contains(resp.Message, "v9.9.9") {
		t.Errorf("message should report the new binary's version: %q", resp.Message)
	}
	// The sanity check must run against the temp file, not the dest.
	if len(r.calls) != 1 || !strings.Contains(r.calls[0], "--version") {
		t.Errorf("expected one --version sanity check, got %v", r.calls)
	}
}

func TestHandleUpdate_ExplicitSHA256(t *testing.T) {
	binary := []byte("pinned-binary")
	srv := fakeRelease(t, binary, false) // no SHA256SUMS published
	dest := updateDest(t)
	sum := sha256.Sum256(binary)

	r := &fakeRunner{respond: func(string) ([]byte, error) { return []byte("v1"), nil }}
	resp := handleUpdate(r, &agent.UpdateRequest{
		DownloadURL: srv.URL + "/release/ezkeel-agent-linux-amd64",
		SHA256:      hex.EncodeToString(sum[:]),
	}, updateOptions{destPath: dest, httpClient: srv.Client(), arch: "amd64"})

	if !resp.OK {
		t.Fatalf("update with explicit checksum failed: %s", resp.Error)
	}
}

func TestHandleUpdate_ChecksumMismatch(t *testing.T) {
	srv := fakeRelease(t, []byte("served-binary"), false)
	dest := updateDest(t)

	r := &fakeRunner{}
	resp := handleUpdate(r, &agent.UpdateRequest{
		DownloadURL: srv.URL + "/release/ezkeel-agent-linux-amd64",
		SHA256:      strings.Repeat("ab", 32), // wrong digest
	}, updateOptions{destPath: dest, httpClient: srv.Client(), arch: "amd64"})

	if resp.OK {
		t.Fatal("checksum mismatch must fail the update")
	}
	if !strings.Contains(resp.Error, "checksum mismatch") {
		t.Errorf("error = %q", resp.Error)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "old-binary" {
		t.Errorf("old binary must be untouched after a failed update: %q", got)
	}
	if len(r.calls) != 0 {
		t.Errorf("an unverified binary must never be executed; ran %v", r.calls)
	}
}

func TestHandleUpdate_RefusesWithoutChecksum(t *testing.T) {
	srv := fakeRelease(t, []byte("served-binary"), false) // no SHA256SUMS, no explicit sha
	dest := updateDest(t)

	resp := handleUpdate(&fakeRunner{}, &agent.UpdateRequest{
		DownloadURL: srv.URL + "/release/ezkeel-agent-linux-amd64",
	}, updateOptions{destPath: dest, httpClient: srv.Client(), arch: "amd64"})

	if resp.OK {
		t.Fatal("update without any obtainable checksum must be refused")
	}
	if !strings.Contains(resp.Error, "refusing unverified update") {
		t.Errorf("error = %q", resp.Error)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "old-binary" {
		t.Errorf("old binary must be untouched: %q", got)
	}
}

func TestHandleUpdate_BrokenBinaryAbortsBeforeSwap(t *testing.T) {
	binary := []byte("looks-fine-but-does-not-run")
	srv := fakeRelease(t, binary, true)
	dest := updateDest(t)

	r := &fakeRunner{
		respond: func(cmd string) ([]byte, error) {
			return []byte("exec format error"), exitErr{"exit status 1"}
		},
	}
	resp := handleUpdate(r, &agent.UpdateRequest{
		DownloadURL: srv.URL + "/release/ezkeel-agent-linux-amd64",
	}, updateOptions{destPath: dest, httpClient: srv.Client(), arch: "amd64"})

	if resp.OK {
		t.Fatal("a binary that fails --version must not be installed")
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "old-binary" {
		t.Errorf("old binary must be untouched: %q", got)
	}
	// The rejected temp file must not be left behind.
	entries, _ := os.ReadDir(filepath.Dir(dest))
	if len(entries) != 1 {
		t.Errorf("temp files left behind: %v", entries)
	}
}

func TestResolveUpdateURL_Precedence(t *testing.T) {
	// Explicit request URL wins over env and version.
	t.Setenv("EZKEEL_AGENT_DOWNLOAD_URL", "https://env.example/agent-{ARCH}")
	got, err := resolveUpdateURL(&agent.UpdateRequest{
		DownloadURL: "https://req.example/agent-{ARCH}",
		Version:     "v0.8.0",
	}, "arm64")
	if err != nil || got != "https://req.example/agent-arm64" {
		t.Errorf("got %q, %v", got, err)
	}

	// Env wins over version when no request URL.
	got, err = resolveUpdateURL(&agent.UpdateRequest{Version: "v0.8.0"}, "amd64")
	if err != nil || got != "https://env.example/agent-amd64" {
		t.Errorf("got %q, %v", got, err)
	}

	t.Setenv("EZKEEL_AGENT_DOWNLOAD_URL", "")

	// Version pin produces the pinned GitHub asset URL.
	got, err = resolveUpdateURL(&agent.UpdateRequest{Version: "v0.8.0"}, "amd64")
	want := "https://github.com/ferax564/ezkeel-cli/releases/download/v0.8.0/ezkeel-agent-linux-amd64"
	if err != nil || got != want {
		t.Errorf("got %q, want %q (%v)", got, want, err)
	}

	// Zero-value request means latest.
	got, err = resolveUpdateURL(&agent.UpdateRequest{}, "arm64")
	want = "https://github.com/ferax564/ezkeel-cli/releases/latest/download/ezkeel-agent-linux-arm64"
	if err != nil || got != want {
		t.Errorf("got %q, want %q (%v)", got, want, err)
	}
}

func TestResolveUpdateURL_RejectsNonHTTP(t *testing.T) {
	if _, err := resolveUpdateURL(&agent.UpdateRequest{DownloadURL: "file:///etc/passwd"}, "amd64"); err == nil {
		t.Error("non-http(s) URL must be rejected")
	}
}

func TestDispatch_UpdateWithNilPayloadMeansLatest(t *testing.T) {
	// dispatch must not nil-panic on an update request without payload —
	// it means "update to latest". We can't reach GitHub in tests, so
	// point the env override at an unroutable URL and just assert the
	// failure is a download error, not a panic or payload error.
	t.Setenv("EZKEEL_AGENT_DOWNLOAD_URL", "http://127.0.0.1:1/nope")
	resp := dispatch(&fakeRunner{}, &agent.Request{Type: agent.CmdUpdate})
	if resp.OK {
		t.Fatal("update against unroutable URL cannot succeed")
	}
	if !strings.Contains(resp.Error, "download") {
		t.Errorf("expected a download error, got %q", resp.Error)
	}
}
