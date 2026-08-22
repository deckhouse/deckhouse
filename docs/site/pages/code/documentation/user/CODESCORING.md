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
- Software Bill of Materials (SBOM) generation
- Platform-side triage

Static (SAST) and dynamic testing (DAST) are not part of this integration.

{% alert level="warning" %}
The CodeScoring integration is a beta feature. It is gated behind the `codescoring_integration` feature flag, which is **disabled by default**. Ask an instance administrator to enable it before you configure the integration.
{% endalert %}

## Integration capabilities

This integration provides the following capabilities:

- Dependency analysis (packages, libraries, versions).
- Vulnerability detection using Common Vulnerabilities and Exposures (CVE) databases (including FSTEC BDU and Kaspersky Open Source Software Threats Data Feed).
- SBOM generation in CycloneDX format.
- Native GitLab reports produced in a single run: "Dependency Scanning", "Code Quality", "JUnit", plus "SARIF" (Static Analysis Results Interchange Format) and a "CycloneDX SBOM".
- Triage and policies (severity thresholds, finding suppression) on the CodeScoring platform side.

## Prerequisites

Before configuring the integration, ensure that:

- A CodeScoring server is deployed (on-premise or SaaS).
- You have obtained an API token from your CodeScoring user profile.
- A GitLab Runner with a `docker` executor is available. The scan job runs inside a `debian:bookworm-slim` container.

{% alert level="info" %}
By default you do not need to install the console agent Johnny manually: it is downloaded automatically from your CodeScoring server using the API token on every scan job run. This behavior is controlled by the "Agent delivery" setting of the integration (see below); you can instead pin a specific agent version or run the scan in your own image that already ships the agent.
{% endalert %}

## Deploying CodeScoring server

For requirements and available CodeScoring server installation methods, refer to the official documentation:

- [System requirements](https://docs.codescoring.ru/en/admin-guide/server-requirements)
- [Installing in Docker](https://docs.codescoring.ru/en/admin-guide/installation)
- [Installing in Kubernetes using a Helm chart](https://docs.codescoring.ru/en/admin-guide/installation-in-k8s)

## Configuring integration in a project

The integration form only activates CodeScoring and stores how to reach the server and how to deliver the scan agent. It does **not** hold any scan options (project name, scanning stage, scan mode, and so on) — those are configured separately, in a scan execution policy (see the "Running the scan" section below).

Connection parameters are configured in project or group settings:

1. Open the project in Deckhouse Code.
1. Navigate to "Settings" → "Integrations" → "CodeScoring".
1. Fill in the parameters:

   | Parameter | Description |
   |-----------|-------------|
   | "Active" | Toggle to enable the integration for this project |
   | "Server URL" | CodeScoring server address. For example, `https://codescoring.example.com` |
   | "API token" | Token from your CodeScoring user profile (stored encrypted and masked) |
   | "Certificate trust" | How TLS is verified when Deckhouse Code connects to the server. Options: "Trust what the job already trusts" (use the system trust store of the runner or image); "Trust the certificate authority below" (paste a CA certificate in PEM format into the field that appears); "Do not verify the certificate" |
   | "Agent delivery" | How the console agent Johnny is provided to the scan job. Options: "Match the CodeScoring server version" (download the agent that matches your server on every run); "Pinned version from our registry" (pull a fixed agent version — set "Agent version"); "Custom image" (run the scan in your own image that already ships the agent — set "Agent image" and, if that registry uses a private CA, its "Agent image registry authority" in PEM format) |

1. Click "Save".
1. Click "Test settings" to validate the URL, token, and TLS trust against the server.

{% alert level="warning" %}
"Do not verify the certificate" relaxes TLS verification **only** for the "Test settings" connection check made from Deckhouse Code. The scan agent in the pipeline always verifies TLS, so for a server with a private or self-signed certificate you must still supply its CA (see below) even when this option is selected.
{% endalert %}

The integration injects the connection and agent-delivery settings into every pipeline automatically (no need to set them in `.gitlab-ci.yml`): the server URL and token (`FE_SCANS_CODESCORING_URL`, `FE_SCANS_CODESCORING_TOKEN`), the selected certificate-trust mode with its CA, and the agent-delivery mode. The scan options themselves are supplied by the scan execution policy.

For a server with a private or self-signed certificate, provide its CA in one of two ways:

- (recommended) the "Certificate trust" → "Trust the certificate authority below" option in the integration form; or
- a **File**-type CI variable `CODESCORING_SSL_FILE` ("Settings" → "CI/CD" → "Variables").

The `codescoring_scan` job exports whichever is set to `SSL_CERT_FILE`, which both `curl` and the Johnny agent trust; when both are present, the CA from the integration takes precedence.

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

- Provides the console agent Johnny according to the "Agent delivery" mode set in the integration (downloaded from the server by token by default; for a self-signed server, using the CA from the integration setting or the `CODESCORING_SSL_FILE` variable).
- Runs the scan once for each stage selected in the policy and submits native GitLab reports. On the project's default branch, the scan is recorded as the default project version.

### Scan options

All scan options live in the policy, in the `codescoring` scan action. The policy editor groups them into two blocks.

#### SCA

| Option | Description |
|--------|-------------|
| Scan stages | One or more stages to associate results with on the platform side: `build`, `dev`, `source`, `stage`, `test`, `prod`, `proxy`. The scan runs once for each selected stage. |
| Scan mode | How the target is scanned. |
| Scan target | Path scanned by the agent (the working directory by default). |
| Ignore paths | Paths excluded from the scan. |
| Resolve with | Package managers used to resolve dependencies. |
| Ask OSA about each component | Query CodeScoring.OSA for a verdict on every resolved component, so a project without OSA Proxy in its build still learns what the proxy would have said. |
| Fail the job on a refused component | Fail the job when OSA refuses at least one component. |
| Fail on an empty result | Fail the job when the scan resolves no components. |
| Extra agent arguments | Additional command-line arguments passed to the agent. |

CodeScoring.OSA is part of the SCA options; it is not a separate module.

#### Secrets

| Option | Description |
|--------|-------------|
| Secret-search engine | Engine used to search for secrets: `gitleaks`, `trufflehog`, or `kingfisher`. |
| Engine binary path | Path to the engine executable. |
| Engine configuration path | Path to the engine configuration file. |
| Ignore paths | Paths excluded from the secret search. |
| Extra secret-search arguments | Additional command-line arguments passed to the engine. |

{% alert level="info" %}
The Secrets block is expected to change in a future Code version; the options above describe its current state.
{% endalert %}

A manual `include` and manual `CODESCORING_*` variables are not required. The integration and the policy provide everything required.

## Reports and viewing results

A single `codescoring_scan` job produces all reports in one run. Deckhouse Code is based on GitLab FOSS, where some EE widgets are absent, so results are surfaced as follows:

| Report | Where to view |
|--------|---------------|
| Tests (JUnit) | "Tests" tab in the pipeline (native) |
| Code Quality | "Code Quality" widget in the merge request (native); carries the severity of SCA findings |
| Dependency-scanning findings (SCA) | Vulnerability report; the same findings also surface in the "Code Quality" widget and the "Tests" tab |
| SBOM (dependency composition) | "Dependency list" page at `/-/security/dependencies` |
| Licenses | "License compliance" page at `/-/security/licenses` |
| SARIF | Uploaded as an artifact (there is no SAST widget in FOSS) |

Findings are also forwarded to DefectDojo when that integration is configured.

The "Dependency list" and "License compliance" pages read the CycloneDX SBOM that the scan uploads; when several analyzers run in the same pipeline, both pages show the union of every analyzer's SBOM.

{% alert level="info" %}
The "Dependency list" and "License compliance" pages are a Deckhouse Code FE implementation. In the upstream GitLab FOSS, the corresponding widgets are EE-only. Both pages appear under the "Secure" section of the project sidebar (when compliance features are enabled for the project) and can also be opened by direct URL.
{% endalert %}

## Policies and blocking

The `codescoring_scan` job **fails the pipeline** when a finding is at or above the `FE_SECURITY_FAIL_ON` severity threshold (`high` by default) — a severity gate. Two more optional gates come from the scan execution policy: "Fail the job on a refused component" (when OSA refuses a component) and "Fail on an empty result" (when the scan resolves no components). Reports are still uploaded regardless (including on failed attempts, via `artifacts:when: always`), so findings are never lost.

Triage and policy configuration (severity thresholds, finding suppression) are handled on the CodeScoring platform side.

## Vulnerability triage

Detected vulnerabilities can be triaged directly in the CodeScoring interface. To do that:

1. Navigate to "SCA" → "Vulnerabilities".
1. Select the status: `Active`, `Confirmed`, `Not affected`, or `False positive`.
1. Fill in the justification and response, compatible with the CycloneDX Vulnerability Exploitability eXchange (VEX) format.

Temporary suppression of findings is available by project, technology, package, license, or CVE.

{% alert level="warning" %}
Currently the CodeScoring agent does not populate the `severity` field in the raw "Dependency Scanning Report" artifact; severity is present in the "Code Quality" and "Tests" surfaces, where the scan template normalizes it. As a result, severity may be missing in tools that read the dependency-scanning artifact directly.
{% endalert %}

## Troubleshooting

### Scan does not start

Check the following:

- The CodeScoring integration is active in project settings (URL and token are set).
- The policy project is linked to the project and contains the `- scan: codescoring` action.
- A `codescoring_scan` job is present in the pipeline and a GitLab Runner with a `docker` executor is available.

### Results do not appear on the Dependency list or License compliance pages

Check the following:

- The `codescoring_scan` job completed and uploaded the `gl-dependency-scanning-report.json` and `gl-sbom.cdx.json` artifacts (collected with `when: always`).
- You are viewing the default branch page (the pages read the latest pipeline's report).

### Code Quality widget is not displayed in the merge request

Verify that `gl-code-quality-report.json` was generated when the job was completed and that it is declared under `artifacts:reports:codequality`.
