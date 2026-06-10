# Changelog

All notable changes to the `ezkeel` CLI are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Agent protocol versioning: `Request`/`Response` envelopes carry
  `protocol_version` (and responses `agent_version`). Missing fields decode as
  `0`, identifying every pre-versioning binary by construction. The client
  warns when an agent speaks an older protocol; the agent rejects requests
  from a *newer* protocol with a structured error instead of mishandling them.
- `version` agent command — side-effect-free version/protocol probe
  (`Client.Version`).
- `update` agent command + `ezkeel server update-agent <name>` — the agent
  downloads a release binary (request URL → host `EZKEEL_AGENT_DOWNLOAD_URL` →
  GitHub release, `{ARCH}` substituted), verifies it against an explicit
  SHA256 or the release's `SHA256SUMS`, sanity-checks it executes, and
  atomically renames it over itself. Updates with no obtainable checksum are
  refused.
- `DBCreateRequest.ro_password`: when set, `db_create` also provisions a
  `<user>_ro` read-only role (CONNECT/USAGE/SELECT + default privileges for
  future tables). Idempotent — re-issuing `db_create` retrofits the role onto
  an existing database.
- Agent install pinning: `ezkeel server add --agent-version v0.x.y` (or
  `EZKEEL_AGENT_VERSION` / `EZKEEL_AGENT_DOWNLOAD_URL` env), and
  `EZKEEL_VERSION` pin support in `install.sh`.
- Agent handler test suite: `exec.Command` is now behind an injectable runner,
  with coverage for deploy/stop/rollback/db_create/db_migrate/update including
  docker failures and the no-`:prev`-image rollback branch.
- `ezkeel.yaml` v1 deploy spec read by `ezkeel up` (overrides framework auto-detect; see `templates/ezkeel.yaml` for the canonical layout).
- `pkg/bootstrap` — reusable Docker + ezkeel-agent installer with an injectable `Runner` interface, `SSHRunner`, and `AliasRunner`.
- `ezkeel server add user@host` runs the full bootstrap by default; `--hetzner` reuses the same SSH path after provisioning + waiting for sshd.

### Changed
- Bumped the CLI Go toolchain to **1.26.3** to pick up standard-library
  security fixes used by release builds.
- `--bootstrap` flag default flipped from `false` to `true`. Pass `--bootstrap=false` to skip on a pre-baked box.
- `--hetzner --bootstrap=false` is now rejected with an explicit error (a fresh Hetzner box requires bootstrap).
- `ezkeel init <project>` now scaffolds an `ezkeel.yaml` next to the existing `workspace.yaml`.
- Default Go build now emits `-o /app/app` so the runner stage's COPY finds the binary regardless of source package layout.
- Default Rust start now references `./app` (the runner-stage path) instead of the builder-only `./target/release/app`.
- Bootstrap commands now prefix privileged steps with `sudo -n` so a non-root SSH user (e.g. `ubuntu`/`debian` on AWS/Vultr/Scaleway) with passwordless sudo can run `ezkeel server add` end-to-end. Root SSH users are unaffected (`sudo -n` is a no-op as root).

### Fixed
- `db_migrate` now parses `migrate_cmd` with shell-quoting rules instead of
  `strings.Fields`, so quoted arguments (e.g.
  `rails runner "User.where(active: true).count"`) survive intact.
- A failed `docker tag` of the previous image during deploy is now surfaced in
  the deploy response message (rollback depends on the `:prev` tag existing);
  previously it was silently ignored.
- `ezkeel env` now resolves to app environment-variable management instead of
  the dev-container `environment` command.
- Generated Dockerfile templates now honor `ezkeel.yaml` `build:` and (for Go/Rust) `start:` overrides instead of hardcoding `npm run build` / `go build ./...` / `cargo build --release` and `CMD ["./app"]`. Next.js, Vite/SPA, and Node SSR templates thread `build:` through; Go and Rust thread both `build:` and `start:` through.
- `ezkeel.yaml` `resources.memory` and `resources.cpus` are now applied at deploy time when the equivalent `--memory` / `--cpus` CLI flag is unset. Previously they were parsed but discarded.
- Bootstrap `agent_download` step single-quotes the agent URL so query strings containing `&` (e.g. presigned asset links) aren't treated as backgrounding operators by the remote login shell.
