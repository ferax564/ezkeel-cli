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
func generateNextjsDockerfile(fr *FrameworkResult) string {
	return fmt.Sprintf(`FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci

FROM node:22-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM node:22-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
EXPOSE %d
CMD ["node", "server.js"]
`, fr.Port)
}

// generateSPADockerfile produces a 2-stage Dockerfile for SPA bundlers
// (e.g. Vite) that output to an outDir such as "dist".
func generateSPADockerfile(fr *FrameworkResult, outDir string) string {
	return fmt.Sprintf(`FROM node:22-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci
COPY . .
RUN npm run build

FROM caddy:2-alpine AS runner
COPY --from=builder /app/%s /srv
EXPOSE 80
CMD ["caddy", "file-server", "--root", "/srv"]
`, outDir)
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
// meta-frameworks (Remix, Nuxt, Astro).
func generateNodeSSRDockerfile(fr *FrameworkResult) string {
	return fmt.Sprintf(`FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci

FROM node:22-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM node:22-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /app ./
RUN npm ci --omit=dev
EXPOSE %d
CMD [%s]
`, fr.Port, shellToCMD(fr.Start))
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
func generateGoDockerfile(fr *FrameworkResult) string {
	return fmt.Sprintf(`FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
RUN go build -o app ./...

FROM alpine:latest AS runner
WORKDIR /app
COPY --from=builder /app/app .
EXPOSE %d
CMD ["./app"]
`, fr.Port)
}

// generateRustDockerfile produces a 2-stage Dockerfile for Rust applications.
func generateRustDockerfile(fr *FrameworkResult) string {
	return fmt.Sprintf(`FROM rust:1.80-slim AS builder
WORKDIR /app
COPY . .
RUN cargo build --release

FROM debian:bookworm-slim AS runner
WORKDIR /app
COPY --from=builder /app/target/release/app .
EXPOSE %d
CMD ["./app"]
`, fr.Port)
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
