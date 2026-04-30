# Changelog

All notable changes to the `ezkeel` CLI are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `ezkeel.yaml` v1 deploy spec read by `ezkeel up` (overrides framework auto-detect; see `templates/ezkeel.yaml` for the canonical layout).
- `pkg/bootstrap` — reusable Docker + ezkeel-agent installer with an injectable `Runner` interface, `SSHRunner`, and `AliasRunner`.
- `ezkeel server add user@host` runs the full bootstrap by default; `--hetzner` reuses the same SSH path after provisioning + waiting for sshd.

### Changed
- `--bootstrap` flag default flipped from `false` to `true`. Pass `--bootstrap=false` to skip on a pre-baked box.
- `--hetzner --bootstrap=false` is now rejected with an explicit error (a fresh Hetzner box requires bootstrap).
- `ezkeel init <project>` now scaffolds an `ezkeel.yaml` next to the existing `workspace.yaml`.
