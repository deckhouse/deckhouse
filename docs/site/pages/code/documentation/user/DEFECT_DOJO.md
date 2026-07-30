---
title: "DefectDojo integration"
menuTitle: DefectDojo
force_searchable: true
description: "Configure DefectDojo vulnerability management integration for Deckhouse Code."
permalink: en/code/documentation/user/defect-dojo.html
weight: 50
---

DefectDojo integration lets you collect vulnerabilities from Deckhouse Code security reports in a single vulnerability management system.

![DefectDojo integration form](/images/code/defect_dojo_integration_form_en.png)

## Prerequisites

Before you enable the integration, make sure that:

- You have a reachable DefectDojo instance.
- You have a DefectDojo API token with access to scan import.
- You have the **Maintainer** role in the Deckhouse Code project.
- Your CI pipeline produces supported security artifacts.

## Enable DefectDojo integration

To configure the integration:

1. Open your project in Deckhouse Code.
1. Go to **Settings** → **Integrations**.
1. Open **DefectDojo**.
1. Fill in the integration fields:
   - **URL**.
   - **API token**.
   - **Product name** (optional, defaults to project full path).
   - **Product type name**.
   - **Engagement name** (optional, defaults to branch name).
   - **Minimum severity**.
   - **Auto-create context** (`auto_create_context`).
   - **Imported findings (active)** (`findings_active`).
   - **Verified findings** (`findings_verified`).
   - **Close old findings** (`close_old_findings`).
1. Click **Save changes**.

## Validate connection settings

Click **Test settings** on the DefectDojo integration page to verify that Deckhouse Code can reach DefectDojo and use the provided token.

## How automatic upload works

After each pipeline build finishes with security artifacts, Deckhouse Code automatically uploads the scan reports to DefectDojo using the `reimport-scan` API.

The integration uploads reports from these scanners:

- SAST
- Secret detection
- Dependency scanning
- Container scanning
- DAST

## Mapping used for DefectDojo entities

By default, Deckhouse Code maps uploaded data as follows:

- **Product** = project full path (or custom Product name).
- **Engagement** = branch name (or custom Engagement name).
- **Test** = CI job name.
- **test_title** = CI job name.

## Import defaults

Deckhouse Code sends import parameters according to integration settings:

- Minimum severity threshold.
- `auto_create_context`.
- Active state for imported findings.
- Verified state for imported findings.
- `close_old_findings`.

## Secure CI credentials

If you use built-in CI variables for DefectDojo integration (`DD_URL` and `DD_TOKEN`), mark them as **masked** and **protected**.

## Findings in DefectDojo

The screenshot below shows findings imported into DefectDojo.

![DefectDojo findings](/images/code/defect_dojo_findings_en.png)
