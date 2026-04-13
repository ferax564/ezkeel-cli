# EZKeel CLI

**Deploy any repo to your own server in one command.**

[![CI](https://github.com/ferax564/ezkeel-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/ferax564/ezkeel-cli/actions/workflows/ci.yml)
[![Release](https://github.com/ferax564/ezkeel-cli/actions/workflows/release.yml/badge.svg)](https://github.com/ferax564/ezkeel-cli/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

```bash
ezkeel up github.com/user/my-app
# Detects framework, builds Docker image, deploys, prints URL.
```

EZKeel auto-detects your framework (Next.js, Vite, FastAPI, Go, Rails, etc.), generates a Dockerfile, provisions databases, and deploys to your own VPS with auto-SSL — all in one command.

## Install

```bash
curl -fsSL https://ezkeel.com/install.sh | sh
```

Or build from source:

```bash
git clone https://github.com/ferax564/ezkeel-cli
cd ezkeel-cli
go build -o ezkeel ./cmd/ezkeel
```

## Quick start

```bash
# Add your VPS (one-time)
ezkeel server add --host 168.119.x.x --domain deploy.mysite.com

# Deploy any repo
ezkeel up github.com/user/my-app
# => Live at https://my-app.deploy.mysite.com
```

## What it does

- **One-command deploy** — `ezkeel up` detects, builds, and deploys. Zero config.
- **16 frameworks** — Next.js, Nuxt, Remix, Astro, Vite, Express, Hono, Fastify, FastAPI, Django, Flask, Go, Rust, Rails, static HTML, Dockerfile passthrough.
- **Auto-SSL + custom domains** — Caddy handles TLS. Add domains with `ezkeel domain add`.
- **Dedicated Postgres per app** — Provisioned user + database, `DATABASE_URL` injected at deploy.
- **Bring your VPS** — Any provider. Paste SSH key; EZKeel handles Docker, TLS, routing.
- **Auto-provision** — `ezkeel server add --hetzner` creates a VPS and configures everything.
- **Zero-downtime + rollback** — Health-checked deploys. `ezkeel rollback` reverts instantly.
- **Built-in diagnostics** — `ezkeel doctor` checks SSH, Docker, agent, DNS, disk space.

## Commands

```bash
# Deploy
ezkeel up [repo-url]           # Deploy any app (--dry-run to preview)
ezkeel server add --host <ip>  # Add a VPS (--hetzner to auto-provision)
ezkeel apps                    # List deployed apps
ezkeel logs <app>              # Stream app logs
ezkeel down <app>              # Remove an app
ezkeel rollback <app>          # Roll back to previous deployment
ezkeel env set <app> K=V       # Set env vars
ezkeel backup <app>            # Backup app database
ezkeel doctor                  # Check system health
ezkeel domain add <app> <dom>  # Add custom domain
ezkeel ci setup                # Generate CI workflow (--github for GitHub Actions)

# Agents
ezkeel agent list              # List available agents
ezkeel agent run <name> <p>    # One-shot agent execution
ezkeel agent chat <name>       # Interactive agent session

# Platform
ezkeel platform install        # Deploy Forgejo + Infisical + Caddy stack
ezkeel init <project>          # Create new project
ezkeel clone <project>         # Clone with full setup
ezkeel secrets inject <env>    # Inject secrets into shell
ezkeel ai claude "prompt"      # Launch AI tool with injected secrets
```

See [`docs/cli-reference.html`](https://ezkeel.com/cli-reference.html) for the full reference.

## Requirements

- **Local**: Go 1.26+ (to build), SSH client
- **VPS**: Any Linux box with SSH access. EZKeel installs Docker + Caddy + its agent on first `ezkeel server add`.

## Managed alternative

Don't want to run your own VPS? [`app.ezkeel.com`](https://app.ezkeel.com) is the hosted dashboard — same CLI, zero infrastructure.

## Contributing

Issues and PRs welcome. Run `go test ./... && go vet ./...` before opening a PR.

## License

MIT. See [LICENSE](LICENSE).
