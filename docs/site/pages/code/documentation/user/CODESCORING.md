---
title: "CodeScoring Integration"
menuTitle: CodeScoring
force_searchable: true
description: Configure CodeScoring SCA/OSA scanner integration in Deckhouse Code for dependency and vulnerability analysis
permalink: en/code/documentation/user/codescoring.html
lang: en
weight: 90
---

CodeScoring is a Software Composition Analysis (SCA/OSA) tool for auditing third-party dependencies for vulnerabilities, license risks, and security policy violations.

{% alert level="info" %}
The CodeScoring integration in Deckhouse Code covers SCA/OSA scenarios: dependency analysis, vulnerability detection, SBOM generation, and policy enforcement.
SAST and DAST are not part of this integration.
{% endalert %}

## Integration capabilities

- Dependency analysis (packages, libraries, versions).
- Vulnerability detection using CVE databases (including FSTEC BDU and Kaspersky OSS feed).
- SBOM generation in CycloneDX format.
- Security policy enforcement with blocking or warning CI behavior.
- Native report output in GitLab Dependency Scanning and Code Quality formats for MR widget display.

## Prerequisites

Before configuring the integration, ensure that:

- A CodeScoring server is deployed (on-prem or SaaS).
- You have obtained an API token from your CodeScoring user profile.
- The Johnny agent is available in the CI environment (Docker image or binary).

For server deployment details, refer to the official documentation: [docs.codescoring.ru](https://docs.codescoring.ru/on-premise/).

## Configuring the integration in a project

CodeScoring connection parameters are configured in project settings.

1. Open the project in Deckhouse Code.
2. Navigate to **Settings** → **Integrations**.
3. Find the **CodeScoring** section and click **Configure**.
4. Fill in the connection parameters:

| Parameter | Description |
|-----------|-------------|
| **Server URL** | CodeScoring server address, e.g. `https://codescoring.example.com` |
| **API token** | Token from your CodeScoring user profile |
| **Project name** | Project name in CodeScoring (defaults to repository name) |
| **Scan stage** | CI stage for result association: `build`, `dev`, `stage`, `test`, `prod` (default: `build`) |
| **Enable integration** | Toggle to activate integration for this project |

5. Click **Save**.

## CI pipeline configuration

After configuring the integration, include the CodeScoring template in the project's `.gitlab-ci.yml`.

### Including the template

```yaml
include:
  - project: "deckhouse/code/gitlab-custom"
    file: ".gitlab/ci/includes/codescoring.gitlab-ci.yml"

variables:
  CODESCORING_ENABLED: "true"
  CODESCORING_URL: $CODESCORING_URL         # set in CI/CD Variables
  CODESCORING_TOKEN: $CODESCORING_TOKEN     # set as a masked variable
  CODESCORING_PROJECT: $CI_PROJECT_NAME
  CODESCORING_SCAN_STAGE: "build"
  CODESCORING_POLICY_MODE: "blocking"       # or "warning"
```

Set `CODESCORING_URL` and `CODESCORING_TOKEN` via **Settings → CI/CD → Variables**, marking the token as `Masked`.

### Pipeline stages

The integration adds the following stages:

| Job | Stage | Description |
|-----|-------|-------------|
| `codescoring-sbom` | `.pre` | Generates CycloneDX SBOM. Artifact is passed to downstream jobs |
| `codescoring-dependency-scan` | `security` | Dependency analysis, outputs GitLab Dependency Scanning Report |
| `codescoring-code-quality-scan` | `security` | Code quality checks, outputs GitLab Code Quality Report |
| `codescoring-build-scan` | `security` | Build artifact analysis (optional, requires `CODESCORING_BUILD_PATH`) |

Scan jobs run **in parallel** after SBOM generation, reducing overall scan time.

## SBOM pre-stage

Before scanning, a SBOM (Software Bill of Materials) is automatically generated in CycloneDX format:

- SBOM captures the exact dependency composition at build time.
- A single SBOM is reused by multiple scan jobs.
- The artifact is available for reuse by other tools.

If a SBOM artifact already exists from a previous stage, regeneration is skipped.

## Policy modes

### Blocking mode

The pipeline fails on policy violation (exit code 1):

```yaml
variables:
  CODESCORING_POLICY_MODE: "blocking"
```

Recommended for protected branches and release environments.

### Warning mode

Results are published as warnings without stopping the pipeline:

```yaml
variables:
  CODESCORING_POLICY_MODE: "warning"
```

Recommended for pilot rollouts or feature branches.

## Displaying results in Merge Requests

After scanning, results appear in MR widgets:

- **Security scanning** — detected vulnerabilities with CVE details, severity, and recommendations.
- **Code Quality** — quality metric violations.

Widgets appear automatically when `gl-dependency-scanning-report.json` and `gl-code-quality-report.json` artifacts are present.

## Vulnerability triage

Detected vulnerabilities can be triaged directly in the CodeScoring interface:

- Navigate to **SCA → Vulnerabilities**.
- Set status: `Active`, `Confirmed`, `Not affected`, `False positive`.
- Fill in justification and response (compatible with CycloneDX VEX format).

Temporary suppression of findings is available by project, technology, package, license, or CVE.

## CodeScoring server deployment

For self-hosted installation, refer to:

- [Docker Compose](deployment-docker.html) — single-server deployment.
- [Kubernetes/Helm](https://docs.codescoring.ru/on-premise/kubernetes/) — production environment.

For system requirements, see [docs.codescoring.ru/on-premise/requirements/](https://docs.codescoring.ru/on-premise/requirements/).

## Troubleshooting

### Scan does not start

Check:

- `CODESCORING_ENABLED` is set to `"true"`.
- `CODESCORING_URL` and `CODESCORING_TOKEN` are set and accessible to the runner.
- The template is included in `.gitlab-ci.yml`.

### Pipeline blocked on policy violation

This is expected behavior in `blocking` mode. To temporarily disable blocking:

- Switch to `CODESCORING_POLICY_MODE: "warning"`, or
- Resolve the violation through triage in the CodeScoring interface.

### Security widgets not displayed in MR

Check:

- `gl-dependency-scanning-report.json` and `gl-code-quality-report.json` artifacts are created.
- The `artifacts.reports` section in the job configuration is correct.
- The job completed (artifacts are collected even on failure with `when: always`).
