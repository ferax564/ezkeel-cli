# {{PROJECT_NAME}} — Agents Guide

## Shared Context

All agents working on {{PROJECT_NAME}} share the following context:

- **Workspace config**: `workspace.yaml` defines visibility, secrets, AI models, and CI
- **AI persona**: Defined in `workspace.yaml` under `ai.persona`
- **Secrets**: Injected at runtime from Infisical; never read from disk
- **MCP servers**: Filesystem MCP is available for reading the project tree

## Project Structure

```
{{PROJECT_NAME}}/
├── src/           # Main source code (public)
├── tests/         # Test suite (public)
├── docs/          # Documentation (public)
├── plans/         # Planning documents (private)
├── internal/      # Internal notes and research (private)
├── .claude/       # Claude-specific config (private)
├── .forgejo/      # CI workflow definitions
├── .devcontainer/ # Dev container configuration
├── workspace.yaml # EZKeel workspace config (private)
├── CLAUDE.md      # AI assistant instructions
└── AGENTS.md      # This file
```

## Rules

1. **Visibility**: Respect the `visibility` section in `workspace.yaml`. Never expose
   contents of `private_paths` in public outputs, summaries, or logs.
2. **Secrets**: Never print, log, or commit secret values. Always use environment
   variable references (e.g., `$ANTHROPIC_API_KEY`).
3. **Tests first**: Run the test suite before and after any non-trivial change.
4. **Minimal footprint**: Make the smallest change that satisfies the requirement.
   Avoid refactoring unrelated code in the same PR.
5. **Ask before deleting**: Do not delete files or directories without explicit
   instruction from the user.
