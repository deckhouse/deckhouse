---
name: run-docs-website
description: Runs the Deckhouse documentation website locally via `docs/site/Makefile` and werf. Use when the user wants to run the docs, preview the documentation site, start the docs server, or open the documentation website locally.
---

# Run Deckhouse Kubernetes Platform Documentation Website

## Quick start

1. **Go to** `docs/site/`:
   ```bash
   cd docs/site
   ```

2. **Run the docs in development mode**:
   ```bash
   make dev
   ```

3. **Open** `http://localhost/products/kubernetes-platform/documentation/v1/` in a browser.

4. **Stop** when done:
   ```bash
   make down
   ```

The first run can take several minutes while images are building.

## External module workflow

Use this workflow when you want to preview documentation from an external module repository together with the local portal and see changes immediately.

1. **Go to** `docs/site/`:
   ```bash
   cd docs/site
   ```

2. **Run the external module workflow**:
   ```bash
   make external-module MODULE_PATH=/path/to/module
   ```

3. **Optional arguments**:
   - `CHANNEL=alpha` by default
   - `MODULE_VERSION=v0.1.0` by default
   - `USE_LOCALHOST_REPO=1` to use `localhost:4999/docs`

4. **Open** `http://localhost/products/kubernetes-platform/documentation/v1/` in a browser.

5. **Stop** when done:
   ```bash
   make down
   ```

This workflow uses `hugo server` in `--renderStaticToDisk` mode, so changes in the external module repository are reflected automatically.

## Other targets (from docs/site/)

| Target | Use case |
|--------|----------|
| `make up` | Start docs in watch mode that rebuilds on commit |
| `make dev` | Start docs in DEV watch mode that rebuilds on documentation changes |
| `make external-module MODULE_PATH=/path/to/module` | Watch an external module docs and run the local portal |
| `USE_LOCALHOST_REPO=1 make dev` | Use `localhost:4999/docs` and start the local Docker registry automatically |
| `USE_LOCALHOST_REPO=1 make external-module MODULE_PATH=/path/to/module` | Same as above for the external module workflow |
| `USE_LOCALHOST_REPO=1 make up` | Same as above, but for non-DEV watch mode |
| `make clean` | Clean werf artifacts when using `USE_LOCALHOST_REPO=1` |
| `make regenerate-menu` | Regenerate navigation menu from API |
| `make regenerate-metadata` | Regenerate embedded modules metadata for local development |
| `make down` | Stop containers, remove networks, and stop the local registry |

## Notes

- The docs site is rendered by Jekyll with Liquid.
- Running `make registry` manually is optional; it is only needed when you explicitly use `USE_LOCALHOST_REPO=1`.
