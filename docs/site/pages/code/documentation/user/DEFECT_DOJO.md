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
1. Select the "Active" checkbox.
1. Fill in the "Connection" fields:
   - "DefectDojo URL": Base URL of the DefectDojo instance.
   - "API token": DefectDojo API token.
1. Fill in the "Import defaults" fields, which are applied when scan reports are imported:
   - "Product name" (optional): Product name in DefectDojo. Defaults to the project full path.
   - "Product type name" (optional): Product type in DefectDojo, used when a product is auto-created.
   - "Engagement name" (optional): Engagement name in DefectDojo. Defaults to the branch or tag the pipeline ran on.
   - "Minimum severity": Findings below this severity are not imported.
   - "Auto-create context": Create the product and engagement in DefectDojo if they do not exist.
   - "Imported findings": Mark imported findings as active.
   - "Verified findings": Mark imported findings as verified.
   - "Close old findings": Close the findings that are missing from the new report.
1. Click "Save changes".

The integration is also available at the group and instance level: in that case, the projects inherit the settings.

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

Reports are imported by Deckhouse Code itself, so the pipeline requires no extra job and no DefectDojo credentials in CI variables.

### Mapping DefectDojo entities

By default, Deckhouse Code maps uploaded data as follows:

- **Product** = project full path (or the specified "Product name").
- **Engagement** = branch or tag the pipeline ran on (or the specified "Engagement name").
- **Test** = CI job name that produced the report.

Because a test is pinned per scanner job, scanners do not close each other's findings.

### Import parameters

When importing scanning results, Deckhouse Code sends the "Import defaults" values to DefectDojo, along with the pipeline ID, commit SHA, and branch or tag name of the imported report.

## Keeping the API token safe

- Use a dedicated DefectDojo service account whose permissions are limited to importing scanning results.
- The token is stored encrypted and is never shown again after it is saved: to replace it, enter a new one in the "Enter new API token" field.
- Configure the integration at the group or instance level to reuse a single token across projects instead of copying it into each one.
