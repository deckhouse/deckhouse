# oss.yaml werf helpers tests

This directory contains small integration-style tests for werf go-template helpers implemented in [` .werf/defines/oss-yaml.tmpl`](../../.werf/defines/oss-yaml.tmpl:1).

The tests render minimal werf configs using `werf config render --dev` and assert that:
- **render-ok** cases render successfully
- **render-fail** cases fail to render

## How to run

From the repository root:

```bash
go test ./testing/werf_defines/oss-yaml
```

The test runner expects a `werf` binary at `./bin/werf`. If it is missing, the test tries to build it via `make bin/werf`. If build fails, tests will be skipped.

## Structure

- `render-ok/<case-name>/`
  - `werf.yaml` — a minimal werf config that uses one or more helpers and fails explicitly if the helper result is not as expected
  - `oss.yaml` — per-case fixture file

- `render-fail/<case-name>/`
  - `werf.yaml` — a minimal werf config that is expected to fail rendering (e.g. ambiguous version selection, overlap, missing id)
  - `oss.yaml` — per-case fixture file

## How helpers are wired into fixtures

Each fixture directory is treated as an independent werf project.

Before rendering a case, the Go test creates `.<caseDir>/.werf/defines/oss-yaml.tmpl` as a **symlink** to the repository helper template [` .werf/defines/oss-yaml.tmpl`](../../.werf/defines/oss-yaml.tmpl:1). This guarantees that tests always run against the current helper implementation.

On Windows, the test falls back to copying the file (symlinks may require elevated privileges).

## Adding a new test case

1. Pick whether it should render:
   - success: add a directory under `render-ok/`
   - failure: add a directory under `render-fail/`

2. Create two files in the case directory:
   - `oss.yaml`
   - `werf.yaml`

3. In `werf.yaml` ensure the helper reads the local fixture:

```yaml
{{- $_ := set . "OssYamlPath" "oss.yaml" -}}
project: test-oss-yaml-helper
configVersion: 1
---
image: test
from: alpine:3.20
shell:
  install:
  - echo ok
```

4. Add assertions in `werf.yaml`:
   - For *render-ok* cases: compute the helper output and call `fail` if it is not equal to the expected result.
   - For *render-fail* cases: it is enough to call the helper in a way that should fail.

Notes:
- Werf `fromYaml` expects a map (document) at top-level. If you need to parse an array returned by a helper, wrap it first:

```yaml
{{- $raw := printf "data:\n%s" (include "get_oss_versions_by_id" (list "example-service" $)) | fromYaml -}}
{{- $versions := index $raw "data" -}}
```

## Debugging

If rendering fails unexpectedly, run the same render command manually in a case directory:

```bash
cd testing/werf_defines/oss-yaml/render-ok/<case>
../../../../bin/werf config render --dev --debug-templates
```

The test runner also sets `WERF_GITERMINISM_CONFIG` to [`werf-giterminism.yaml`](../../werf-giterminism.yaml:1) to make rendering possible from nested fixture directories.
