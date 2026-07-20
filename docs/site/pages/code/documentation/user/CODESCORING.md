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
The CodeScoring integration in Deckhouse Code covers SCA/OSA scenarios: dependency analysis, vulnerability detection, SBOM generation, and platform-side triage.
SAST and DAST are not part of this integration.
{% endalert %}

## Integration capabilities

- Dependency analysis (packages, libraries, versions).
- Vulnerability detection using CVE databases (including FSTEC BDU and Kaspersky OSS feed).
- SBOM generation in CycloneDX format.
- Native GitLab reports produced in a single run: Dependency Scanning, Code Quality, JUnit, plus SARIF and a CycloneDX SBOM.
- Triage and policies (severity thresholds, finding suppression) on the CodeScoring platform side.

## Prerequisites

Before configuring the integration, ensure that:

- A CodeScoring server is deployed (on-prem or SaaS).
- You have obtained an API token from your CodeScoring user profile.
- A GitLab Runner with a `docker` executor is available — the scan job runs inside a `debian:bookworm-slim` container.

For server deployment details, refer to the official documentation: [docs.codescoring.ru](https://docs.codescoring.ru/on-premise/).

{% alert level="info" %}
You do **not** need to install the Johnny agent manually: the scan job downloads it from your CodeScoring server using the API token on every run.
{% endalert %}

## Configuring the integration in a project

CodeScoring connection parameters are configured in project (or group) settings.

1. Open the project in Deckhouse Code.
1. Navigate to **Settings** → **Integrations**.
1. Find the **CodeScoring** section and open it.
1. Fill in the connection parameters:

| Parameter | Description |
|-----------|-------------|
| **Active** | Toggle to enable the integration for this project |
| **Server URL** | CodeScoring server address, e.g. `https://codescoring.example.com` |
| **API token** | Token from your CodeScoring user profile (stored encrypted, masked) |
| **CA certificate** | Optional PEM CA certificate — for a CodeScoring server with a self-signed certificate |
| **Project name** | Project name in CodeScoring (defaults to the repository slug) |
| **Scan stage** | Stage used to associate results on the platform side (default: `build`) |

1. Click **Save**.

The integration automatically injects the CI variables `FE_SCANS_CODESCORING_URL`, `FE_SCANS_CODESCORING_TOKEN`, `FE_SCANS_CODESCORING_CA_CERT`, `FE_SCANS_CODESCORING_PROJECT`, and `FE_SCANS_CODESCORING_SCAN_STAGE` — you do not set them manually in `.gitlab-ci.yml`.

## Running the scan (scan-execution policy)

The scanner is injected into the pipeline through a **scan-execution policy**, not a manual `include`.

1. In the security policy project, add the `codescoring` action to `policy.yml`:

   ```yaml
   scan_execution_policy:
   - name: CodeScoring on every pipeline
     enabled: true
     rules:
     - type: pipeline
       branches: ["*"]
     actions:
     - scan: codescoring
   ```

1. Link the policy project to the target project: **Settings** → **Security policy**.

After that, every pipeline automatically gains a **`codescoring_scan`** job (stage `fe-security-scanner`) that:

- downloads the Johnny agent from the CodeScoring server (by token; with the supplied CA certificate for self-signed servers);
- scans the working directory and emits native GitLab reports.

A manual `include` and manual `CODESCORING_*` variables are **not required** — the integration and the policy provide everything.

## Reports and where to view results

A single `codescoring_scan` job produces all reports in one run. Deckhouse Code is based on GitLab FOSS, where some EE widgets are absent, so results are surfaced as follows:

| Report | Where to view |
|--------|---------------|
| Tests (JUnit) | pipeline **Tests** tab (native) |
| Code Quality | **Code Quality** widget in the Merge Request (native) |
| Dependency Scanning | **CodeScoring** page: `/-/security/codescoring` |
| SBOM (dependency composition) | **Dependency list** page: `/-/security/dependencies` |
| Licenses | **License compliance** page: `/-/security/licenses` |
| SARIF | uploaded as an artifact (no SAST widget in FOSS) |

{% alert level="info" %}
The Dependency Scanning, Dependency list, and License compliance pages are a Deckhouse Code FE implementation: in upstream GitLab FOSS the corresponding widgets are EE-only. The pages are currently reached by direct URL (a sidebar menu entry is planned).
{% endalert %}

## Policies and blocking

The `codescoring_scan` job is non-blocking: it always completes successfully and uploads reports (including on failed attempts, via `artifacts:when: always`) without failing the pipeline.

Policy configuration (40 criteria, severity thresholds, triage) and any blocking decision are handled on the CodeScoring platform side. Hard pipeline blocking on a policy violation is a separate scan-execution-policy setup and is not enabled in the current template.

## Vulnerability triage

Detected vulnerabilities can be triaged directly in the CodeScoring interface:

- Navigate to **SCA → Vulnerabilities**.
- Set status: `Active`, `Confirmed`, `Not affected`, `False positive`.
- Fill in justification and response (compatible with CycloneDX VEX format).

Temporary suppression of findings is available by project, technology, package, license, or CVE.

{% alert level="warning" %}
Currently the CodeScoring agent does not populate `severity` in the Dependency Scanning Report (only in Code Quality), so severity may appear as `unknown` on the CodeScoring page.
{% endalert %}

## CodeScoring server deployment

For a self-hosted installation, use the vendor's official documentation:

- [Docker installation](https://docs.codescoring.ru/on-premise/docker/).
- [Kubernetes/Helm installation](https://docs.codescoring.ru/on-premise/kubernetes/).
- [System requirements](https://docs.codescoring.ru/on-premise/requirements/).

## Troubleshooting

### Scan does not start

Check:

- The **CodeScoring** integration is active in project settings (URL and token are set).
- The policy project is linked to the project and contains `- scan: codescoring`.
- A `codescoring_scan` job is present in the pipeline and a Runner with a `docker` executor is available.

### Results do not appear on the CodeScoring / Dependency list / License compliance pages

Check:

- The `codescoring_scan` job completed and uploaded the `gl-dependency-scanning-report.json` and `gl-sbom.cdx.json` artifacts (collected with `when: always`).
- You are viewing the default branch page (the pages read the latest pipeline's report).

### The Code Quality widget is not displayed in the Merge Request

Verify that the job produced `gl-code-quality-report.json` and that it is declared under `artifacts:reports:codequality`.
