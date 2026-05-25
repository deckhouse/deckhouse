---
name: run-documentation-module
description: Runs the Deckhouse documentation module locally from the repository root. Use when the user wants to build or start `documentation/web`, preview the documentation module, or run the docs module workflow.
---

# Run Deckhouse Documentation Module

## Prerequisites

- Run from the **repo root** (`/home/kar/deckhouse/repo` or your clone).

## 1. Build the documentation module

```bash
make build FOCUS=documentation/web
```

## 2. Open the module locally

- Open `http://localhost:81`.

## Notes

- No manual export of environment variables is required for this workflow.
- Use the repository `Makefile` workflow instead of `werf run documentation/web`.
- The first build can take several minutes.
- The documentation content is rendered by Jekyll with Liquid.
