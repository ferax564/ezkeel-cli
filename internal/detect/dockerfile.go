package detect

import (
	"fmt"
	"regexp"
	"strings"
)

// GenerateDockerfile returns a Dockerfile content string for the given
// FrameworkResult. It returns an empty string for unknown frameworks or
// when a real Dockerfile already exists (FrameworkDockerfile).
func GenerateDockerfile(fr *FrameworkResult) string {
	switch fr.Framework {
	case FrameworkNextjs:
		return generateNextjsDockerfile(fr)
	case FrameworkVite:
		return generateSPADockerfile(fr, "dist")
	case FrameworkExpress, FrameworkHono, FrameworkFastify:
		return generateNodeServerDockerfile(fr)
	case FrameworkRemix, FrameworkNuxt, FrameworkAstro:
		return generateNodeSSRDockerfile(fr)
	case FrameworkFastAPI, FrameworkDjango, FrameworkFlask:
		return generatePythonDockerfile(fr)
	case FrameworkGo:
		return generateGoDockerfile(fr)
	case FrameworkRust:
		return generateRustDockerfile(fr)
	case FrameworkRails:
		return generateRailsDockerfile(fr)
	case FrameworkStatic:
		return generateStaticDockerfile()
	default:
		return ""
	}
}

// generateNextjsDockerfile produces a 3-stage Dockerfile for Next.js apps
// that use output: 'standalone'.
//
// Note: only the BUILD step is parameterized via fr.Build. CMD stays
// hardcoded to `node server.js` because that path is dictated by the
// standalone-output runner stage layout — overriding it via spec.start
// would point at a path that doesn't exist on the runner image.
func generateNextjsDockerfile(fr *FrameworkResult) string {
	build := fr.Build
	if build == "" {
		build = "npm run build"
	}
	return fmt.Sprintf(`FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci

FROM node:22-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN %s

FROM node:22-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
EXPOSE %d
CMD ["node", "server.js"]
`, build, fr.Port)
}

// generateSPADockerfile produces a 2-stage Dockerfile for SPA bundlers
// (e.g. Vite) that output to an outDir such as "dist".
//
// Note: only the BUILD step is parameterized via fr.Build. CMD stays
// hardcoded to caddy file-server because the runner image is Caddy and
// the static asset path is fixed at /srv.
func generateSPADockerfile(fr *FrameworkResult, outDir string) string {
	build := fr.Build
	if build == "" {
		build = "npm run build"
	}
	return fmt.Sprintf(`FROM node:22-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci
COPY . .
RUN %s

FROM caddy:2-alpine AS runner
COPY --from=builder /app/%s /srv
EXPOSE 80
CMD ["caddy", "file-server", "--root", "/srv"]
`, build, outDir)
}

// generateNodeServerDockerfile produces a single-stage Dockerfile for
// Node.js server frameworks (Express, Hono, Fastify).
//
// When fr.Build is set (e.g. `npm run build` for a TypeScript Express
// server), this honors it: install ALL deps with `npm ci` (not
// `npm ci --omit=dev`), run the build, then ship. Without this, a
// TS-Express spec would scaffold `start: node dist/index.js` but the
// build step that produces `dist/` would never run — the container
// would crash at boot trying to require a non-existent path. Some
// runtime libraries (tsx, ts-node) also live in devDependencies and
// are imported at runtime, which `--omit=dev` would strip.
//
// When fr.Build is empty (plain JS), keep `--omit=dev` for a smaller
// runtime image — there's no build step to run.
func generateNodeServerDockerfile(fr *FrameworkResult) string {
	npmInstall := "npm ci --omit=dev"
	buildStep := ""
	if fr.Build != "" {
		// Need full deps: the build step likely uses tsc / esbuild /
		// etc. from devDependencies, and some runtimes (tsx, ts-node)
		// require devDependencies at runtime.
		npmInstall = "npm ci"
		buildStep = fmt.Sprintf("RUN %s\n", fr.Build)
	}
	return fmt.Sprintf(`FROM node:22-alpine
WORKDIR /app
COPY package.json package-lock.json* ./
RUN %s
COPY . .
%sEXPOSE %d
CMD %s
`, npmInstall, buildStep, fr.Port, shellToCMD(fr.Start))
}

// generateNodeSSRDockerfile produces a 3-stage Dockerfile for SSR Node.js
// meta-frameworks (Remix, Nuxt, Astro). Honors fr.Build and fr.Start so
// spec overrides flow through.
func generateNodeSSRDockerfile(fr *FrameworkResult) string {
	build := fr.Build
	if build == "" {
		build = "npm run build"
	}
	return fmt.Sprintf(`FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci

FROM node:22-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN %s

FROM node:22-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /app ./
RUN npm ci --omit=dev
EXPOSE %d
CMD %s
`, build, fr.Port, shellToCMD(fr.Start))
}

// generatePythonDockerfile produces a Dockerfile for Python frameworks
// (FastAPI, Django, Flask).
func generatePythonDockerfile(fr *FrameworkResult) string {
	buildStep := ""
	if fr.Build != "" {
		buildStep = fmt.Sprintf("RUN %s\n", fr.Build)
	}
	return fmt.Sprintf(`FROM python:3.13-slim
WORKDIR /app
COPY requirements.txt* ./
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
%sEXPOSE %d
CMD %s
`, buildStep, fr.Port, shellToCMD(fr.Start))
}

// generateGoDockerfile produces a 2-stage Dockerfile for Go applications.
//
// Convention: the build command must produce a binary at /app/app so
// the runner stage's COPY --from=builder /app/app /app/app finds it.
// The default Build (DefaultsFor(FrameworkGo)) uses `-o /app/app`. Custom
// spec build commands should follow the same convention.
func generateGoDockerfile(fr *FrameworkResult) string {
	build := fr.Build
	if build == "" {
		// Mirror DefaultsFor(FrameworkGo). `-o /app/app .` (NOT `./...`)
		// is required: combining `-o <file>` with `./...` fails for
		// multi-package modules. Repos with main at ./cmd/<name> override
		// Build in ezkeel.yaml.
		build = "go build -o /app/app ."
	}
	cmd := shellToCMD(fr.Start)
	if cmd == "" || cmd == "[]" {
		cmd = `["./app"]`
	}
	return fmt.Sprintf(`FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
RUN %s

FROM alpine:latest AS runner
WORKDIR /app
COPY --from=builder /app/app /app/app
EXPOSE %d
CMD %s
`, build, fr.Port, cmd)
}

// generateRustDockerfile produces a 2-stage Dockerfile for Rust applications.
//
// Convention: the build step must leave a binary at
// /app/target/release/app — i.e. the crate must be named "app", or the
// spec must override Build with a command that produces that path
// (e.g. `cargo build --release && cp target/release/<crate> target/release/app`).
// The runner stage's COPY then moves it to /app/app, and Start = "./app"
// invokes it. Users with a differently-named binary should override
// both build: and start: in ezkeel.yaml.
func generateRustDockerfile(fr *FrameworkResult) string {
	build := fr.Build
	if build == "" {
		build = "cargo build --release"
	}
	cmd := shellToCMD(fr.Start)
	if cmd == "" || cmd == "[]" {
		cmd = `["./app"]`
	}
	return fmt.Sprintf(`FROM rust:1.80-slim AS builder
WORKDIR /app
COPY . .
RUN %s

FROM debian:bookworm-slim AS runner
WORKDIR /app
COPY --from=builder /app/target/release/app /app/app
EXPOSE %d
CMD %s
`, build, fr.Port, cmd)
}

// generateRailsDockerfile produces a Dockerfile for Ruby on Rails applications.
func generateRailsDockerfile(fr *FrameworkResult) string {
	return fmt.Sprintf(`FROM ruby:3.3-slim
WORKDIR /app
RUN apt-get update -qq && apt-get install -y build-essential libpq-dev nodejs && rm -rf /var/lib/apt/lists/*
COPY Gemfile Gemfile.lock* ./
RUN bundle install
COPY . .
EXPOSE %d
CMD %s
`, fr.Port, shellToCMD(fr.Start))
}

// generateStaticDockerfile produces a single-stage Dockerfile that serves
// static files with Caddy.
func generateStaticDockerfile() string {
	return `FROM caddy:2-alpine
COPY . /srv
EXPOSE 80
CMD ["caddy", "file-server", "--root", "/srv"]
`
}

// shellToCMD returns the payload that follows `CMD ` in a Dockerfile.
//
// For simple space-separated commands it emits Docker exec-form
// (`["binary", "arg"]`) so the binary runs as PID 1 and signals are
// forwarded correctly.
//
// For commands containing shell metacharacters — quotes, redirects,
// pipes, `&&`, `||`, `;`, `$` — Fields-splitting would shred the
// command into broken JSON tokens (e.g.
// `["sh", "-c", "\"python", "manage.py", "&&"]`). Those route to
// shell-form (the bare command string after `CMD `), which Docker
// implicitly wraps with `sh -c` and preserves shell semantics.
//
// An empty input returns the empty array `[]` — preserves prior
// behavior for callers that supply their own fallback (Go/Rust).
func shellToCMD(cmd string) string {
	if cmd == "" {
		return "[]"
	}
	if needsShell(cmd) {
		// Shell-form: docker wraps with `sh -c`. Emit the command
		// string verbatim so quoting and redirects survive.
		return cmd
	}
	parts := strings.Fields(cmd)
	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = `"` + p + `"`
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// needsShell reports whether cmd contains shell metacharacters that
// make a naive Fields-split unsafe. When true, the caller must emit
// shell-form CMD instead of exec-form.
func needsShell(cmd string) bool {
	metaChars := []string{`"`, `'`, "`", "&&", "||", ";", "|", ">", "<", "$"}
	for _, m := range metaChars {
		if strings.Contains(cmd, m) {
			return true
		}
	}

	// A leading env-var assignment (FIRST whitespace-delimited token of
	// the form NAME=value) means the command relies on a shell to bind
	// those vars before exec'ing the rest. Exec-form would treat them
	// as the binary name and fail. We deliberately do NOT trigger on
	// later tokens like `--host=0.0.0.0` — only the first token's
	// shape matters.
	if firstToken := firstField(cmd); firstToken != "" {
		if envAssignmentRe.MatchString(firstToken) {
			return true
		}
	}

	return false
}

// firstField returns the first whitespace-separated token, or "" if
// the input has no non-whitespace.
func firstField(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

// envAssignmentRe matches a leading shell env-var assignment like
// `NODE_ENV=production` or `_FOO=bar` but NOT `--flag=value` or
// `not_a_var=...` (lowercase start). Conservative: env names must
// start with an uppercase letter or underscore, then uppercase
// letters/digits/underscores, then `=` and any value.
var envAssignmentRe = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*=`)
