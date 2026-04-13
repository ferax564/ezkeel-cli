# {{PROJECT_NAME}}

## Overview

{{PROJECT_NAME}} is a project managed with EZKeel. This file provides guidance for
AI assistants working in this codebase.

## Development

- Run tests before committing: `make test` or the project's equivalent
- Run the linter and formatter before committing
- Keep commits small and focused; prefer one logical change per commit
- Write commit messages in the imperative mood ("Add feature" not "Added feature")

## Conventions

- Source code lives in `src/`
- Tests live in `tests/` and mirror the structure of `src/`
- Documentation lives in `docs/`
- Internal/private planning documents live in `plans/` and `internal/`
- Secrets are managed via Infisical; never hard-code credentials
- Use the workspace.yaml to configure AI behavior and project visibility
