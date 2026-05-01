package detect

import (
	"fmt"
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
func generateNodeServerDockerfile(fr *FrameworkResult) string {
	return fmt.Sprintf(`FROM node:22-alpine
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci --omit=dev
COPY . .
EXPOSE %d
CMD [%s]
`, fr.Port, shellToCMD(fr.Start))
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
CMD [%s]
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
CMD [%s]
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
		build = "go build -o /app/app ./..."
	}
	cmd := shellToCMD(fr.Start)
	if cmd == "" {
		cmd = `"./app"`
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
CMD [%s]
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
	if cmd == "" {
		cmd = `"./app"`
	}
	return fmt.Sprintf(`FROM rust:1.80-slim AS builder
WORKDIR /app
COPY . .
RUN %s

FROM debian:bookworm-slim AS runner
WORKDIR /app
COPY --from=builder /app/target/release/app /app/app
EXPOSE %d
CMD [%s]
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
CMD [%s]
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

// shellToCMD converts a shell command string (e.g. "node index.js") into a
// Docker JSON exec-form argument list (e.g. `"node", "index.js"`).
// It performs a simple whitespace split — sufficient for the well-known
// start commands produced by DetectFramework.
func shellToCMD(cmd string) string {
	if cmd == "" {
		return ""
	}
	parts := strings.Fields(cmd)
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += `"` + p + `"`
	}
	return result
}
