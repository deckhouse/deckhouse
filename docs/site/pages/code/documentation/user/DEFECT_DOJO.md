---
title: "DefectDojo integration"
menuTitle: DefectDojo integration
force_searchable: true
description: "Configure DefectDojo vulnerability management integration for Deckhouse Code."
permalink: en/code/documentation/user/defect-dojo.html
weight: 50
---

This integration lets you automatically import security report results from Deckhouse Code into DefectDojo, a single vulnerability management system.

![DefectDojo integration form](/images/code/defect_dojo_integration_form_en.png)

## Prerequisites

Before configuring the integration, make sure that:

- A DefectDojo instance is available.
- You obtained a DefectDojo API token eligible to import scanning results.
- The **Maintainer** role has been configured in the Deckhouse Code project.
- Your CI pipeline produces supported security artifacts.

## Configuring DefectDojo integration

To configure the integration:

1. Open your project in Deckhouse Code.
1. Go to "Settings" → "Integrations" → "DefectDojo".
1. Fill in the integration fields:
   - "URL": DefectDojo instance address.
   - "API token": DefectDojo API token.
   - "Product name" (optional): Product name in DefectDojo. By default, it's the project full path.
   - "Product type name": Product type in DefectDojo.
   - "Engagement name" (optional): By default, it's the branch name.
   - "Minimum severity": Minimum severity of imported findings.
   - "Auto-create context" (`auto_create_context`): Automatically create missing entities in DefectDojo.
   - "Imported findings (active)" (`findings_active`): Mark imported findings as active.
   - "Verified findings" (`findings_verified`): Mark imported findings as verified.
   - "Close old findings" (`close_old_findings`): Close findings that are missing from the new report.
1. Click "Save changes".

### Validating connection

To verify that Deckhouse Code can reach DefectDojo and use the provided API token, click "Test settings" on the integration page.

## Using the integration

### Automatic importing of scanning results

After each CI pipeline build finishes with scanning reports, Deckhouse Code automatically imports them to DefectDojo using the `reimport-scan` API.

The following scanning type results are supported:

- SAST
- Secret detection
- Dependency scanning
- Container scanning
- DAST

### Mapping DefectDojo entities

By default, Deckhouse Code maps uploaded data as follows:

- **Product** = project full path (or specified "Product name").
- **Engagement** = branch name (or specified "Engagement name").
- **Test** = CI job name.
- **test_title** = CI job name.

### Import parameters

When importing scanning results, Deckhouse Code sends parameters to DefectDojo according to integration settings:

- "Minimum severity" (minimum severity of imported findings)
- `auto_create_context`
- `findings_active`
- `findings_verified`
- `close_old_findings`

## Secure CI credentials

If you use built-in `DD_URL` and `DD_TOKEN` CI variables for DefectDojo integration, mark them as **masked** and **protected**.
