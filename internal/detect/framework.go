package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Framework represents a detected application framework.
type Framework string

const (
	FrameworkDockerfile Framework = "dockerfile"
	FrameworkNextjs     Framework = "nextjs"
	FrameworkVite       Framework = "vite"
	FrameworkExpress    Framework = "express"
	FrameworkHono       Framework = "hono"
	FrameworkFastify    Framework = "fastify"
	FrameworkRemix      Framework = "remix"
	FrameworkNuxt       Framework = "nuxt"
	FrameworkAstro      Framework = "astro"
	FrameworkFastAPI    Framework = "fastapi"
	FrameworkDjango     Framework = "django"
	FrameworkFlask      Framework = "flask"
	FrameworkGo         Framework = "go"
	FrameworkRust       Framework = "rust"
	FrameworkRails      Framework = "rails"
	FrameworkStatic     Framework = "static"
	FrameworkUnknown    Framework = "unknown"
)

// FrameworkResult contains the detected framework and recommended build/start commands.
type FrameworkResult struct {
	Framework  Framework
	Build      string // e.g. "npm run build"
	Start      string // e.g. "node .next/standalone/server.js"
	Port       int    // e.g. 3000
	Dockerfile string // "Dockerfile" if exists, "auto" if generated
}

// packageJSON is a minimal representation of a Node.js package.json file.
type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// hasDep returns true if the given dependency name is found in either
// Dependencies or DevDependencies.
func (p *packageJSON) hasDep(name string) bool {
	if _, ok := p.Dependencies[name]; ok {
		return true
	}
	if _, ok := p.DevDependencies[name]; ok {
		return true
	}
	return false
}

// DetectFramework scans dir and returns the best-matched FrameworkResult.
// Detection priority:
//  1. Dockerfile
//  2. package.json (Node frameworks)
//  3. requirements.txt / pyproject.toml (Python)
//  4. go.mod (Go)
//  5. Cargo.toml (Rust)
//  6. Gemfile with "rails" (Rails)
//  7. index.html (static)
//  8. FrameworkUnknown
func DetectFramework(dir string) (*FrameworkResult, error) {
	// 1. Dockerfile takes priority
	if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err == nil {
		return &FrameworkResult{
			Framework:  FrameworkDockerfile,
			Dockerfile: "Dockerfile",
		}, nil
	}

	// 2. Node.js frameworks via package.json
	if result, ok := detectNode(dir); ok {
		return result, nil
	}

	// 3. Python frameworks via requirements.txt / pyproject.toml
	if result, ok := detectPython(dir); ok {
		return result, nil
	}

	// 4. Go via go.mod
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return &FrameworkResult{
			Framework: FrameworkGo,
			// Output to /app/app so the runner stage's
			// COPY --from=builder /app/app /app/app finds the binary
			// regardless of the source package layout. Without an explicit
			// `-o`, `go build ./...` lands the binary at `/app/<pkg>`,
			// which the COPY can't anticipate.
			Build:      "go build -o /app/app ./...",
			Start:      "./app",
			Port:       8080,
			Dockerfile: "auto",
		}, nil
	}

	// 5. Rust via Cargo.toml
	if _, err := os.Stat(filepath.Join(dir, "Cargo.toml")); err == nil {
		return &FrameworkResult{
			Framework: FrameworkRust,
			Build:     "cargo build --release",
			// Start references the runner-stage path. The Dockerfile's
			// COPY moves target/release/<crate> to /app/app, so the
			// runner CMD invokes ./app — not ./target/release/app, which
			// only exists in the builder stage.
			Start:      "./app",
			Port:       8080,
			Dockerfile: "auto",
		}, nil
	}

	// 6. Rails via Gemfile containing "rails"
	if result, ok := detectRails(dir); ok {
		return result, nil
	}

	// 7. Static HTML via index.html
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
		return &FrameworkResult{
			Framework:  FrameworkStatic,
			Port:       80,
			Dockerfile: "auto",
		}, nil
	}

	// 8. Unknown
	return &FrameworkResult{
		Framework: FrameworkUnknown,
	}, nil
}

// detectNode reads package.json from dir and matches Node.js frameworks.
// Returns the FrameworkResult and true if a match is found.
func detectNode(dir string) (*FrameworkResult, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil, false
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, false
	}

	// SSR / meta-frameworks (checked before SPA bundlers)
	if pkg.hasDep("next") {
		return &FrameworkResult{
			Framework:  FrameworkNextjs,
			Build:      "npm run build",
			Start:      "node .next/standalone/server.js",
			Port:       3000,
			Dockerfile: "auto",
		}, true
	}

	if pkg.hasDep("@remix-run/node") || pkg.hasDep("@remix-run/react") || pkg.hasDep("remix") {
		return &FrameworkResult{
			Framework:  FrameworkRemix,
			Build:      "npm run build",
			Start:      "node ./build/server/index.js",
			Port:       3000,
			Dockerfile: "auto",
		}, true
	}

	if pkg.hasDep("nuxt") || pkg.hasDep("nuxt3") {
		return &FrameworkResult{
			Framework:  FrameworkNuxt,
			Build:      "npm run build",
			Start:      "node .output/server/index.mjs",
			Port:       3000,
			Dockerfile: "auto",
		}, true
	}

	if pkg.hasDep("astro") {
		return &FrameworkResult{
			Framework:  FrameworkAstro,
			Build:      "npm run build",
			Start:      "node ./dist/server/entry.mjs",
			Port:       4321,
			Dockerfile: "auto",
		}, true
	}

	// SPA bundlers
	if pkg.hasDep("vite") || pkg.hasDep("@vitejs/plugin-react") || pkg.hasDep("@vitejs/plugin-vue") {
		return &FrameworkResult{
			Framework:  FrameworkVite,
			Build:      "npm run build",
			Start:      "npx serve dist",
			Port:       5173,
			Dockerfile: "auto",
		}, true
	}

	// Server frameworks
	if pkg.hasDep("express") {
		return &FrameworkResult{
			Framework:  FrameworkExpress,
			Build:      "",
			Start:      "node index.js",
			Port:       3000,
			Dockerfile: "auto",
		}, true
	}

	if pkg.hasDep("hono") {
		return &FrameworkResult{
			Framework:  FrameworkHono,
			Build:      "npm run build",
			Start:      "node dist/index.js",
			Port:       3000,
			Dockerfile: "auto",
		}, true
	}

	if pkg.hasDep("fastify") {
		return &FrameworkResult{
			Framework:  FrameworkFastify,
			Build:      "",
			Start:      "node index.js",
			Port:       3000,
			Dockerfile: "auto",
		}, true
	}

	return nil, false
}

// detectPython checks requirements.txt and pyproject.toml for Python frameworks.
// Returns the FrameworkResult and true if a match is found.
func detectPython(dir string) (*FrameworkResult, bool) {
	// Check requirements.txt
	reqData, reqErr := os.ReadFile(filepath.Join(dir, "requirements.txt"))
	// Check pyproject.toml
	pyData, pyErr := os.ReadFile(filepath.Join(dir, "pyproject.toml"))

	if reqErr != nil && pyErr != nil {
		return nil, false
	}

	combined := string(reqData) + string(pyData)

	// FastAPI must be checked before flask because both could coexist
	if containsWord(combined, "fastapi") {
		return &FrameworkResult{
			Framework:  FrameworkFastAPI,
			Build:      "",
			Start:      "uvicorn main:app --host 0.0.0.0 --port 8000",
			Port:       8000,
			Dockerfile: "auto",
		}, true
	}

	if containsWord(combined, "django") {
		return &FrameworkResult{
			Framework:  FrameworkDjango,
			Build:      "python manage.py collectstatic --noinput",
			Start:      "python manage.py runserver 0.0.0.0:8000",
			Port:       8000,
			Dockerfile: "auto",
		}, true
	}

	if containsWord(combined, "flask") {
		return &FrameworkResult{
			Framework:  FrameworkFlask,
			Build:      "",
			Start:      "flask run --host=0.0.0.0 --port=5000",
			Port:       5000,
			Dockerfile: "auto",
		}, true
	}

	return nil, false
}

// detectRails checks for a Gemfile containing the "rails" gem.
// Returns the FrameworkResult and true if found.
func detectRails(dir string) (*FrameworkResult, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "Gemfile"))
	if err != nil {
		return nil, false
	}

	if containsWord(string(data), "rails") {
		return &FrameworkResult{
			Framework:  FrameworkRails,
			Build:      "bundle exec rake assets:precompile",
			Start:      "bundle exec rails server -b 0.0.0.0 -p 3000",
			Port:       3000,
			Dockerfile: "auto",
		}, true
	}

	return nil, false
}

// containsWord returns true if text contains word as a case-insensitive substring.
func containsWord(text, word string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(word))
}

// DefaultsFor returns the canonical Build/Start/Port for the given
// framework as a *FrameworkResult with Dockerfile set to "auto".
// Returns (nil, false) for FrameworkUnknown, FrameworkDockerfile, and
// any framework not in the table.
//
// The values mirror what DetectFramework returns when it positively
// identifies a framework. Used by spec-rescue paths in `ezkeel up`
// where the user declared `framework:` in ezkeel.yaml without
// specifying Build/Start/Port — without these defaults the generated
// Dockerfile would emit `EXPOSE 0` and an empty `CMD []`.
func DefaultsFor(framework Framework) (*FrameworkResult, bool) {
	switch framework {
	case FrameworkGo:
		return &FrameworkResult{Framework: framework, Build: "go build -o /app/app ./...", Start: "./app", Port: 8080, Dockerfile: "auto"}, true
	case FrameworkRust:
		return &FrameworkResult{Framework: framework, Build: "cargo build --release", Start: "./app", Port: 8080, Dockerfile: "auto"}, true
	case FrameworkStatic:
		return &FrameworkResult{Framework: framework, Port: 80, Dockerfile: "auto"}, true
	case FrameworkNextjs:
		return &FrameworkResult{Framework: framework, Build: "npm run build", Start: "node .next/standalone/server.js", Port: 3000, Dockerfile: "auto"}, true
	case FrameworkRemix:
		return &FrameworkResult{Framework: framework, Build: "npm run build", Start: "node ./build/server/index.js", Port: 3000, Dockerfile: "auto"}, true
	case FrameworkNuxt:
		return &FrameworkResult{Framework: framework, Build: "npm run build", Start: "node .output/server/index.mjs", Port: 3000, Dockerfile: "auto"}, true
	case FrameworkAstro:
		return &FrameworkResult{Framework: framework, Build: "npm run build", Start: "node ./dist/server/entry.mjs", Port: 4321, Dockerfile: "auto"}, true
	case FrameworkVite:
		return &FrameworkResult{Framework: framework, Build: "npm run build", Start: "npx serve dist", Port: 5173, Dockerfile: "auto"}, true
	case FrameworkExpress:
		return &FrameworkResult{Framework: framework, Start: "node index.js", Port: 3000, Dockerfile: "auto"}, true
	case FrameworkHono:
		return &FrameworkResult{Framework: framework, Build: "npm run build", Start: "node dist/index.js", Port: 3000, Dockerfile: "auto"}, true
	case FrameworkFastify:
		return &FrameworkResult{Framework: framework, Start: "node index.js", Port: 3000, Dockerfile: "auto"}, true
	case FrameworkFastAPI:
		return &FrameworkResult{Framework: framework, Start: "uvicorn main:app --host 0.0.0.0 --port 8000", Port: 8000, Dockerfile: "auto"}, true
	case FrameworkDjango:
		return &FrameworkResult{Framework: framework, Build: "python manage.py collectstatic --noinput", Start: "python manage.py runserver 0.0.0.0:8000", Port: 8000, Dockerfile: "auto"}, true
	case FrameworkFlask:
		return &FrameworkResult{Framework: framework, Start: "flask run --host=0.0.0.0 --port=5000", Port: 5000, Dockerfile: "auto"}, true
	case FrameworkRails:
		return &FrameworkResult{Framework: framework, Build: "bundle exec rake assets:precompile", Start: "bundle exec rails server -b 0.0.0.0 -p 3000", Port: 3000, Dockerfile: "auto"}, true
	}
	return nil, false
}
