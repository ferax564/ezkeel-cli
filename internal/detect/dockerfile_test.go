package detect

import (
	"strings"
	"testing"
)

func TestGenerateDockerfile_NextJS(t *testing.T) {
	fr := &FrameworkResult{
		Framework:  FrameworkNextjs,
		Build:      "npm run build",
		Start:      "node .next/standalone/server.js",
		Port:       3000,
		Dockerfile: "auto",
	}
	got := GenerateDockerfile(fr)

	checks := []string{
		"FROM node:",
		"npm run build",
		"EXPOSE 3000",
		"standalone",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("Next.js Dockerfile missing %q\nGot:\n%s", want, got)
		}
	}
}

func TestGenerateDockerfile_Vite(t *testing.T) {
	fr := &FrameworkResult{
		Framework:  FrameworkVite,
		Build:      "npm run build",
		Start:      "npx serve dist",
		Port:       5173,
		Dockerfile: "auto",
	}
	got := GenerateDockerfile(fr)

	checks := []string{
		"FROM caddy:",
		"npm run build",
		"dist",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("Vite Dockerfile missing %q\nGot:\n%s", want, got)
		}
	}
}

func TestGenerateDockerfile_FastAPI(t *testing.T) {
	fr := &FrameworkResult{
		Framework:  FrameworkFastAPI,
		Build:      "",
		Start:      "uvicorn main:app --host 0.0.0.0 --port 8000",
		Port:       8000,
		Dockerfile: "auto",
	}
	got := GenerateDockerfile(fr)

	checks := []string{
		"FROM python:",
		"requirements.txt",
		"EXPOSE 8000",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("FastAPI Dockerfile missing %q\nGot:\n%s", want, got)
		}
	}
}

func TestGenerateDockerfile_Go(t *testing.T) {
	fr := &FrameworkResult{
		Framework: FrameworkGo,
		// Mirror the canonical default — the build must produce a binary
		// at /app/app for the runner stage's COPY to find it.
		Build:      "go build -o /app/app ./...",
		Start:      "./app",
		Port:       8080,
		Dockerfile: "auto",
	}
	got := GenerateDockerfile(fr)

	checks := []string{
		"FROM golang:",
		"CGO_ENABLED=0",
		"EXPOSE 8080",
		"go build -o /app/app",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("Go Dockerfile missing %q\nGot:\n%s", want, got)
		}
	}
}

func TestGenerateDockerfile_Express(t *testing.T) {
	fr := &FrameworkResult{
		Framework:  FrameworkExpress,
		Build:      "",
		Start:      "node index.js",
		Port:       3000,
		Dockerfile: "auto",
	}
	got := GenerateDockerfile(fr)

	checks := []string{
		"FROM node:",
		"npm ci",
		`"node"`,
		`"index.js"`,
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("Express Dockerfile missing %q\nGot:\n%s", want, got)
		}
	}
}

func TestGenerateDockerfile_Static(t *testing.T) {
	fr := &FrameworkResult{
		Framework:  FrameworkStatic,
		Port:       80,
		Dockerfile: "auto",
	}
	got := GenerateDockerfile(fr)

	checks := []string{
		"FROM caddy:",
		"COPY . /srv",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("Static Dockerfile missing %q\nGot:\n%s", want, got)
		}
	}
}

func TestGenerateDockerfile_Unknown(t *testing.T) {
	fr := &FrameworkResult{
		Framework: FrameworkUnknown,
	}
	got := GenerateDockerfile(fr)
	if got != "" {
		t.Errorf("expected empty string for unknown framework, got:\n%s", got)
	}
}

func TestGenerateDockerfile_ExistingDockerfile(t *testing.T) {
	fr := &FrameworkResult{
		Framework:  FrameworkDockerfile,
		Dockerfile: "Dockerfile",
	}
	got := GenerateDockerfile(fr)
	if got != "" {
		t.Errorf("expected empty string when Dockerfile already exists, got:\n%s", got)
	}
}

func TestGenerateDockerfile_Rust(t *testing.T) {
	fr := &FrameworkResult{
		Framework: FrameworkRust,
		Build:     "cargo build --release",
		// Default Start references the runner-stage path /app/app.
		Start:      "./app",
		Port:       8080,
		Dockerfile: "auto",
	}
	got := GenerateDockerfile(fr)

	checks := []string{
		"FROM rust:",
		"cargo build --release",
		"debian:bookworm-slim",
		"EXPOSE 8080",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("Rust Dockerfile missing %q\nGot:\n%s", want, got)
		}
	}
}

func TestGenerateDockerfile_Rails(t *testing.T) {
	fr := &FrameworkResult{
		Framework:  FrameworkRails,
		Build:      "bundle exec rake assets:precompile",
		Start:      "bundle exec rails server -b 0.0.0.0 -p 3000",
		Port:       3000,
		Dockerfile: "auto",
	}
	got := GenerateDockerfile(fr)

	checks := []string{
		"FROM ruby:",
		"bundle install",
		"EXPOSE 3000",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("Rails Dockerfile missing %q\nGot:\n%s", want, got)
		}
	}
}

func TestGenerateDockerfile_Remix(t *testing.T) {
	fr := &FrameworkResult{
		Framework:  FrameworkRemix,
		Build:      "npm run build",
		Start:      "node ./build/server/index.js",
		Port:       3000,
		Dockerfile: "auto",
	}
	got := GenerateDockerfile(fr)

	checks := []string{
		"FROM node:",
		"npm run build",
		"EXPOSE 3000",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("Remix Dockerfile missing %q\nGot:\n%s", want, got)
		}
	}
}

// TestGenerateDockerfile_HonorsGoBuildOverride verifies that a custom
// Build/Start in the spec actually flows through to the generated Go
// Dockerfile. Before this fix, the template hardcoded `RUN go build -o
// app ./...` and `CMD ["./app"]` regardless of fr.Build / fr.Start, so
// `ezkeel up --dry-run` showed the override but the produced image did
// not.
func TestGenerateDockerfile_HonorsGoBuildOverride(t *testing.T) {
	fr := &FrameworkResult{
		Framework:  FrameworkGo,
		Build:      "go build -tags=embed -o /app/app ./cmd/server",
		Start:      "/app/app --port 9000",
		Port:       9000,
		Dockerfile: "auto",
	}
	got := GenerateDockerfile(fr)
	if !strings.Contains(got, "go build -tags=embed -o /app/app ./cmd/server") {
		t.Errorf("Go Dockerfile missing custom Build override:\n%s", got)
	}
	if !strings.Contains(got, `"/app/app", "--port", "9000"`) {
		t.Errorf("Go Dockerfile missing custom Start override (CMD JSON):\n%s", got)
	}
	if !strings.Contains(got, "EXPOSE 9000") {
		t.Errorf("Go Dockerfile missing EXPOSE 9000:\n%s", got)
	}
}

// TestGenerateDockerfile_HonorsRustBuildOverride mirrors the Go test
// for the Rust template. A user-specified Build that, e.g., enables a
// release feature flag must reach the RUN step.
func TestGenerateDockerfile_HonorsRustBuildOverride(t *testing.T) {
	fr := &FrameworkResult{
		Framework:  FrameworkRust,
		Build:      "cargo build --release --features prod",
		Start:      "./app --bind 0.0.0.0:9000",
		Port:       9000,
		Dockerfile: "auto",
	}
	got := GenerateDockerfile(fr)
	if !strings.Contains(got, "cargo build --release --features prod") {
		t.Errorf("Rust Dockerfile missing custom Build override:\n%s", got)
	}
	if !strings.Contains(got, `"./app", "--bind", "0.0.0.0:9000"`) {
		t.Errorf("Rust Dockerfile missing custom Start override (CMD JSON):\n%s", got)
	}
}

// TestGenerateDockerfile_HonorsNextjsBuildOverride covers the Next.js
// template. CMD stays hardcoded (standalone-output convention) but the
// build step must respect overrides — e.g. `pnpm build` or
// `npm run build:prod`.
func TestGenerateDockerfile_HonorsNextjsBuildOverride(t *testing.T) {
	fr := &FrameworkResult{
		Framework:  FrameworkNextjs,
		Build:      "pnpm build",
		Start:      "node server.js",
		Port:       3000,
		Dockerfile: "auto",
	}
	got := GenerateDockerfile(fr)
	if !strings.Contains(got, "RUN pnpm build") {
		t.Errorf("Next.js Dockerfile missing custom Build override:\n%s", got)
	}
}

// TestGenerateDockerfile_HonorsViteBuildOverride covers the SPA bundler
// template (Vite).
func TestGenerateDockerfile_HonorsViteBuildOverride(t *testing.T) {
	fr := &FrameworkResult{
		Framework:  FrameworkVite,
		Build:      "pnpm build",
		Start:      "npx serve dist",
		Port:       5173,
		Dockerfile: "auto",
	}
	got := GenerateDockerfile(fr)
	if !strings.Contains(got, "RUN pnpm build") {
		t.Errorf("Vite Dockerfile missing custom Build override:\n%s", got)
	}
}

// TestGenerateDockerfile_HonorsRemixBuildOverride covers the Node SSR
// template (Remix/Nuxt/Astro share it).
func TestGenerateDockerfile_HonorsRemixBuildOverride(t *testing.T) {
	fr := &FrameworkResult{
		Framework:  FrameworkRemix,
		Build:      "pnpm build",
		Start:      "node ./build/server/index.js",
		Port:       3000,
		Dockerfile: "auto",
	}
	got := GenerateDockerfile(fr)
	if !strings.Contains(got, "RUN pnpm build") {
		t.Errorf("Node SSR Dockerfile missing custom Build override:\n%s", got)
	}
}

// TestShellToCMDSimpleCommandIsExecForm asserts that simple
// space-separated commands without shell metacharacters emit Docker
// exec-form so the binary runs as PID 1 with proper signal forwarding.
func TestShellToCMDSimpleCommandIsExecForm(t *testing.T) {
	got := shellToCMD("node index.js")
	want := `["node", "index.js"]`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestShellToCMDShellMetaUsesShellForm asserts that commands containing
// shell metacharacters (quotes, pipes, &&, ;, redirects) bypass
// Fields-splitting and pass through verbatim — Docker wraps them with
// `sh -c` so the shell semantics survive. Without this, specs like
// `start: sh -c "python migrate && gunicorn app:app"` would generate
// a Dockerfile with malformed exec-form JSON and fail to build.
func TestShellToCMDShellMetaUsesShellForm(t *testing.T) {
	cases := []string{
		`sh -c "python manage.py migrate && gunicorn app:app"`,
		`node server.js | tee /var/log/app.log`,
		`flask run --port=5000 ; gunicorn`,
		`bash -c 'echo hello && exit 0'`,
		`/app/server > /var/log/app.log 2>&1`,
		`/app/run --env=$NODE_ENV`,
	}
	for _, c := range cases {
		got := shellToCMD(c)
		if strings.HasPrefix(got, "[") {
			t.Errorf("%q got exec-form %q; expected shell-form (verbatim passthrough)", c, got)
		}
		if got != c {
			t.Errorf("%q got %q; expected verbatim shell-form", c, got)
		}
	}
}

// TestShellToCMDEmptyReturnsEmptyArray pins the degenerate input
// behavior: an empty start command yields the empty exec-form array
// `[]`. Callers (Go, Rust) detect this and substitute their own
// runner-stage default (`["./app"]`).
func TestShellToCMDEmptyReturnsEmptyArray(t *testing.T) {
	if got := shellToCMD(""); got != "[]" {
		t.Errorf("got %q, want []", got)
	}
}

// TestNeedsShell directly exercises the metacharacter detector. Any
// false negative here would route a shell-needing command through
// Fields-split and produce malformed JSON in the generated Dockerfile.
func TestNeedsShell(t *testing.T) {
	shellish := []string{
		"a && b",
		"a || b",
		`echo "x"`,
		`echo 'x'`,
		"a | b",
		"a > b",
		"a < b",
		"a ; b",
		"a $X",
		"echo `date`",
	}
	plain := []string{
		"node index.js",
		"uvicorn main:app --host 0.0.0.0 --port 8000",
		"flask run --host=0.0.0.0 --port=5000",
		"python manage.py runserver 0.0.0.0:8000",
		"/app/server",
		"./app",
		"bundle exec rails server -b 0.0.0.0 -p 3000",
	}
	for _, s := range shellish {
		if !needsShell(s) {
			t.Errorf("%q should need shell", s)
		}
	}
	for _, s := range plain {
		if needsShell(s) {
			t.Errorf("%q should NOT need shell", s)
		}
	}
}

// TestGenerateDockerfile_PythonShellStart asserts that a Python spec
// with a shell-form start command (sh -c "...") generates a working
// Dockerfile that uses shell-form CMD. Before this fix, Fields-split
// produced broken JSON like `CMD ["sh", "-c", "\"python", ...]`.
func TestGenerateDockerfile_PythonShellStart(t *testing.T) {
	fr := &FrameworkResult{
		Framework:  FrameworkFastAPI,
		Build:      "",
		Start:      `sh -c "python manage.py migrate && gunicorn app:app"`,
		Port:       8000,
		Dockerfile: "auto",
	}
	got := GenerateDockerfile(fr)
	wantLine := `CMD sh -c "python manage.py migrate && gunicorn app:app"`
	if !strings.Contains(got, wantLine) {
		t.Errorf("Python Dockerfile missing shell-form CMD line %q\nGot:\n%s", wantLine, got)
	}
	// Must NOT emit broken exec-form JSON.
	if strings.Contains(got, `CMD ["sh", "-c", "\"python"`) {
		t.Errorf("Python Dockerfile has broken exec-form JSON:\n%s", got)
	}
}
