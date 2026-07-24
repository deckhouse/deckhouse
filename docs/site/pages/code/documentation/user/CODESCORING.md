---
title: "CodeScoring integration"
menuTitle: CodeScoring
force_searchable: true
description: Configure CodeScoring SCA/OSA scanner integration in Deckhouse Code for dependency and vulnerability analysis
permalink: en/code/documentation/user/codescoring.html
lang: en
weight: 90
---

CodeScoring is a Software Composition Analysis (SCA/OSA) tool for auditing third-party dependencies for vulnerabilities, license risks, and security policy violations.

The CodeScoring integration in Deckhouse Code lets you use CodeScoring features in CI/CD pipelines and covers the following SCA/OSA scenarios:

- Dependency analysis
- Vulnerability detection
- SBOM generation
- Platform-side triage

Static (SAST) and dynamic testing (DAST) are not part of this integration.

## Integration capabilities

This integration provides the following capabilities:

- Dependency analysis (packages, libraries, versions).
- Vulnerability detection using CVE databases (including Kaspersky Open Source Software Threats Data Feed).
- SBOM generation in CycloneDX format.
- Native GitLab reports produced in a single run: "Dependency Scanning", "Code Quality", "JUnit", plus "SARIF" and a "CycloneDX SBOM".
- Triage and policies (severity thresholds, finding suppression) on the CodeScoring platform side.

## Prerequisites

Before configuring the integration, ensure that:

- A CodeScoring server is deployed (on-premise or SaaS).
- You have obtained an API token from your CodeScoring user profile.
- A GitLab Runner with a `docker` executor is available. The scan job runs inside a `debian:bookworm-slim` container.

{% alert level="info" %}
You do not need to install the console agent Johnny manually. It's downloaded automatically from your CodeScoring server using the API token on every scan job run.
{% endalert %}

## Deploying CodeScoring server

For requirements and available CodeScoring server installation methods, refer to the official documentation:

- [System requirements](https://docs.codescoring.ru/en/admin-guide/server-requirements)
- [Installing in Docker](https://docs.codescoring.ru/en/admin-guide/installation)
- [Installing in Kubernetes using a Helm chart](https://docs.codescoring.ru/en/admin-guide/installation-in-k8s)

## Configuring integration in a project

CodeScoring connection parameters are configured in project or group settings:

1. Open the project in Deckhouse Code.
1. Navigate to "Settings" → "Integrations" → "CodeScoring".
1. Fill in the connection parameters:

   | Parameter | Description |
   |-----------|-------------|
   | "Active" | Toggle to enable the integration for this project |
   | "Server URL" | CodeScoring server address. For example, `https://codescoring.example.com` |
   | "API token" | Token from your CodeScoring user profile (stored encrypted and masked) |
   | "Project name" | Project name in CodeScoring (optional; defaults to the `group-project` path, `CI_PROJECT_PATH_SLUG`) |
   | "Scan stage" | Stage used to associate results on the platform side (`build` by default) |

1. Click "Save".

The integration automatically injects the following CI variables into the pipeline (no need to set them manually in `.gitlab-ci.yml`):

- `FE_SCANS_CODESCORING_URL`
- `FE_SCANS_CODESCORING_TOKEN`
- `FE_SCANS_CODESCORING_PROJECT` (when "Project name" is set)
- `FE_SCANS_CODESCORING_SCAN_STAGE`

For a server with a self-signed certificate, the CA is set **manually** as a **File**-type CI variable `CODESCORING_SSL_FILE` ("Settings" → "CI/CD" → "Variables"). The `codescoring_scan` job exports it to `SSL_CERT_FILE`, which both `curl` and the Johnny agent trust. This is the only variable you set by hand; the rest of the `FE_SCANS_CODESCORING_*` variables are injected by the integration.

## Running the scan

The scanner is injected into the pipeline through a **scan-execution policy**, not a manual `include`.

To configure automatic scanning, do the following:

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

1. Link the policy project to the target project in "Settings" → "Security policy".

After that, every pipeline automatically gains a **`codescoring_scan`** job (stage `fe-security-scanner`) that:

- Downloads the console agent Johnny from the CodeScoring server (by token; for a self-signed server, using the CA from the `CODESCORING_SSL_FILE` variable).
- Scans the working directory and submits native GitLab reports.

A manual `include` and manual `CODESCORING_*` variables are not required. The integration and the policy provide everything required.

## Reports and viewing results

A single `codescoring_scan` job produces all reports in one run. Deckhouse Code is based on GitLab FOSS, where some EE widgets are absent, so results are surfaced as follows:

| Report | Where to view |
|--------|---------------|
| Tests (JUnit) | "Tests" tab in the pipeline (native) |
| Code Quality | "Code Quality" widget in the merge request (native) |
| Dependency Scanning | "CodeScoring" page at `/-/security/codescoring` |
| SBOM (dependency composition) | "Dependency list" page at `/-/security/dependencies` |
| Licenses | "License compliance" page at `/-/security/licenses` |
| SARIF | Uploaded as an artifact (there is no SAST widget in FOSS) |

{% alert level="info" %}
The "Dependency Scanning", "Dependency list", and "License compliance" pages are a Deckhouse Code FE implementation. In the upstream GitLab FOSS, the corresponding widgets are EE-only. The pages are currently reached by direct URL (a sidebar menu entry is coming in one of the future Code versions).
{% endalert %}

## Policies and blocking

The `codescoring_scan` job **fails the pipeline** when a finding is at or above the `FE_SECURITY_FAIL_ON` severity threshold (`high` by default) — a severity gate. Reports are still uploaded regardless (including on failed attempts, via `artifacts:when: always`), so findings are never lost.

Policy configuration (40 criteria, severity thresholds, triage) is handled on the CodeScoring platform side.

## Vulnerability triage

Detected vulnerabilities can be triaged directly in the CodeScoring interface. To do that:

1. Navigate to "SCA" → "Vulnerabilities".
1. Select the status: `Active`, `Confirmed`, `Not affected`, or `False positive`.
1. Fill in the justification and response (compatible with CycloneDX VEX format).

Temporary suppression of findings is available by project, technology, package, license, or CVE.

{% alert level="warning" %}
Currently the CodeScoring agent does not populate the `severity` field in the "Dependency Scanning Report" (only in "Code Quality"), so severity may appear as `unknown` on the CodeScoring page.
{% endalert %}

## Troubleshooting

### Scan does not start

Check the following:

- The CodeScoring integration is active in project settings (URL and token are set).
- The policy project is linked to the project and contains the `- scan: codescoring` action.
- A `codescoring_scan` job is present in the pipeline and a GitLab Runner with a `docker` executor is available.

### Results do not appear on the CodeScoring, Dependency list, or License compliance pages

Check the following:

- The `codescoring_scan` job completed and uploaded the `gl-dependency-scanning-report.json` and `gl-sbom.cdx.json` artifacts (collected with `when: always`).
- You are viewing the default branch page (the pages read the latest pipeline's report).

### Code Quality widget is not displayed in the merge request

Verify that `gl-code-quality-report.json` was generated when the job was completed and that it is declared under `artifacts:reports:codequality`.
