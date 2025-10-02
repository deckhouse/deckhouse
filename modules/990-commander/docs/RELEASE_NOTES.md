---
title: "Release Notes"
---

## v1.11.7

### New Features

* Added a new preset template `Deckhouse Virtualization Platform on bare metal` (installation of DVP on static nodes).

### Bug Fixes

* Added a missing Container Registry configuration to the preset template `DKP on Deckhouse Virtualization Platform`, which had prevented the installation of clusters from this template.

### Chore

* Fixed discovered vulnerabilities in the module components.
* The preset template `Deckhouse Virtualization Platform` has been renamed to `DKP on Deckhouse Virtualization Platform` (creating DKP clusters in DVP).
* The Cluster Administration Interface has been updated to the current version 1.39.3.
* Information on cluster joining and detaching operations has been added to the module documentation, as well as information about how to transfer a cluster from one workspace to another.

## v1.11.6

### New Features

* Added a new preset template for DVP (creating DKP clusters in DVP).

### Bug Fixes

* A close button was added to error pop-ups.
* Fixed an issue where it was impossible to select the same catalog entry multiple times in different settings in the cluster form.
* Fixed a bug with restoring a catalog from an archive.
* Resolved an issue where changing the cluster template would trigger a pop-up with incoming changes.
* Fixed a bug where default values for parameters were not displayed in the template editor.

### Chore

* Cluster administration interface updated to the current version 1.39.1.

## v1.11.5

### New Features

* A "Dual List" type has been added to the template builder for the "Choose from Catalog" parameter, allowing all catalog entries to be selected with a single click in the cluster creation form.

### Bug Fixes

* Fixed an issue where an empty resource group sync mode selector was displayed in the cluster form saved without installation.

## v1.11.4

### Bug Fixes

* Fixed issues with the operation of the TCP tunnel on the connector side when using Citrix NetScaler for filtering and balancing traffic between master and slave clusters.

## v1.11.3

### New Features

* Added the ability to specify multiple PostgreSQL servers in the module configuration when using an external database in cluster mode.

### Bug Fixes

#### Interface

* Solved the issue with the sudden appearance of the "Session expired" message by adding a session periodic update mechanism.
* Fixed an error that prevented changing the cluster name.
* Resolved the issue with loading the "Cluster Administration" tab, which occurred when a user was a member of a large number of groups (over 300) on the authentication server.
* Fixed a problem where logs were partially disappearing during cluster joining, making it difficult to determine the cause of the join operation error.
* Fixed the display of image assets in the Cluster Administration interface.
* Cluster Administration interface is updated to version 1.38.4.

#### Cluster Manager

* Fixed a critical error in starting the cluster manager when the managing cluster is deployed from a private container registry with a self-signed TLS certificate.
* Resolved an issue where, upon joining, the deprecated module `deckhouse-commander-agent` was enabled in the subordinate cluster instead of `commander-agent`.

#### Other

* Fixed errors that occurred during the first run of migrations on an empty database.

### Chore

* Added recommendations for using external PostgreSQL installation in production environments to the module documentation.

## v1.11.2

### Bug Fixes

#### Interface

* Fixed the issue with creating and editing a cluster from a template using input parameters of the "Simple values selection" type with multiple selection enabled.
* Resolved the problem with incorrect version numbering of imported templates.
* Addressed the situation where a request for confirmation of destructive changes was displayed for a cluster that had a deletion operation initiated.
* Validation errors for input parameters with a "Validation regular expression" are now displayed correctly.
* In the template editing form on the resource group page, the problem with blocking the "Allow manage in cluster" toggle has been fixed.
* Fixed the display of user visit times in the user list.

### Chore

* Updated module component dependencies to fix discovered vulnerabilities.
* Updated the DKP UI interface to version 1.37.4 on the cluster's "Administration" tab.
* Updated documentation on configuring change history log collection using the `log-shipper` module.

## v1.11.1

### New Features

* RBAC: The built-in DKP UI (console) on the cluster 'Administration' tab will adapt to the user's Commander rights.

### Bug Fixes

* Fixed an error when connecting clusters to Deckhouse Commander after updating to 1.11
* Minor fixes

## v1.11.0

### New Features

#### Interface

* Added support for quick rollback of cluster operations.
* Experimental support for dark theme. The switch is in the user settings.

#### Module

* Added support for authorization using TLS certificates (`cert auth`) when connecting to the PostgreSQL database.

#### Templates

* Added HTTP proxy configuration parameters to pre-installed templates.

### Bug Fixes

#### Interface

* The cluster no longer gets stuck in the "Pending Deletion" status when trying to delete in manual change application mode.
* Fixed an issue that made it impossible to detach a cluster that was in "Pending Changes" or "Destructive Changes" states.
* Cluster reconciliations are now performed according to the configured interval. There was an issue where the interval was ignored if the cluster was in the "Unknown" state.
* 500 server errors are now correctly displayed in the UI.
* Now, if an input parameter is optional and a minimum length is specified, empty values are allowed.
* Cluster configuration (YAML) is now normalized and correctly formatted when rendering a template.
* When deleting records from the directory, the entries now disappear from the list immediately without the need to refresh the page.
* Fixed the translation for workspace deletion errors.
* The restriction on editing input parameters of a cluster in "Deletion Error" status has been disabled. This could prevent the user from changing the necessary input parameters for cluster deletion.

#### Templates

* Removed extra spaces in SSH parameters that caused incorrect formatting of the SSH key in the cluster configuration.
* In the static cluster template, the default user was renamed (`commander` instead of `deckhouse`).

#### Module

* Increased the allowable size of headers so that tokens with a large number of groups do not cause an error with code 429.
* Fixed the issue of substituting an empty server CA certificate in template parameters.
* Fixed the custom TLS certificate copying hook.

### Chore

* Commander will no longer operate over HTTP. From now on, operation over HTTPS is mandatory for security purposes. If HTTPS is disabled in the cluster, the Commander's web interface will be unavailable.
* Basic auth support has been disabled. The `user-authn` module or external authentication must be used for user identification.
* Added a page with descriptions of changes for each release (Release Notes) to the documentation.
* Manual changes of dhctl server manifests in the management cluster by the user will now be forcibly overwritten by the cluster manager.
* Reduced timeouts for some phases of cluster installation.
* Built-in DKP UI (console) updated to version 1.37.2
* The module's component images have been updated to fix vulnerabilities.

## v1.10.7

### Bug Fixes

* Fixed an issue that caused the "Administration" tab to be non-functional in clusters if any clusters were in the "Creation Error" state.
* Corrected an error that occurred when attempting to upgrade a cluster to a new template version on the "Clusters" tab of the template page.
* Resolved a problem that prevented detaching a cluster if manual mode for applying changes was selected in its settings.

### Chore

* The built-in DKP UI (console) has been updated to version 1.35.1

## v1.10.6

### Bug Fixes

* Fixed an error when importing catalogs in "Inventory".

### Chore

* Embedded DKP UI (console) updated to version 1.35.0

## v1.10.5

### New Features

#### Cluster Manager

* Added support for working through a proxy in the management cluster. The cluster manager now uses proxy settings for connections if they are specified in the management cluster configuration.

### Bug Fixes

#### Interface

* Fixed a validation error that occurred when changing the template or modifying input parameters on the cluster page and switching between tabs with resource groups.
* Fixed an issue with infinite reconnection to the server via websocket after restarting Redis.
* Fixed the display of the "Retry Operation" button when a destructive change to the cluster fails.
* On the Kubernetes tab of the cluster, corrected the display of the API resource group applied by the agent (only the version was shown).
* Fixed an error that occurred when attempting to delete a workspace with archived clusters.

### Chore

* Updated module component dependencies to fix discovered vulnerabilities.

## v1.10.4

### Bug Fixes

#### Interface

* Fixed an error in importing a template by pasting text.
* The left button (original) in the configuration cluster changes display now works correctly.
* Fixed an error that occurred when using hidden input parameters in the template.
* Navigating to the deleting cluster page in the "Clusters" tab in the cluster template now works correctly.
* The ability to choose a value from another directory has been removed for immutable input parameters on the cluster page.

#### Cluster Manager

* During the validation of cluster configuration, go-template rendering errors are now displayed; previously, only validation errors were shown.

#### Templates

* The default value (license-token) for the `dockerLogin` parameter has been removed from all pre-installed templates, which caused non-obvious cluster configuration validation errors when the `dockerPassword` field was not filled in.

## v1.10.3

### Bug Fixes

* Fixed template cluster rendering errors related to the presence of extra document delimiters in the YAMLs, as well as in cases when an empty group of resources is rendered.
* During the migration of pre-installed templates, previously archived templates in the installation will not be migrated.

## v1.10.2

### New Features

#### Templates

* A `sudoPassword` parameter has been added to all pre-installed templates.

### Bug Fixes

#### Interface

* Fixed an issue with switching between clusters in the Administration tab, where data from the first cluster was displayed when switching to the Administration tab of another cluster.
* Sometimes the "Overview" tab failed to load in the Administration tab. This issue has been resolved.
* Fixed a 403 error when trying to open the API documentation.
* Default values from the template are now placed in input parameters when creating a cluster, as before.
* Unnecessary text in the unique value validation error in the inventory has been removed.
* Parameters with password type in the catalog schema are now hidden from the record header.
* Fixed display of values when changing the field type in the catalog data schema.
* Fixed errors in browser console when creating a cluster.
* Application cluster graphs no longer extend off-screen on monitors with low resolution.
* Clusters in the list are no longer duplicated. Previously, clicking "Show N new clusters" would display already existing clusters in the list.
* Fixed duplication of the template version when saving the form simultaneously by different users.
* The "Overview" tab is no longer hidden when deleting a cluster.
* Added clarification for errors related to incorrect catalog identifier names, specifying what exactly is incorrect.
* Fixed content jumps on the page when zooming at 90% in the browser.

#### Cluster Manager

* Fixed an issue where the connector was loading the CPU to 100% due to missing connections from the agent.

#### Module

* Fixed an issue with launching Redis due to a missing serviceAccount.

#### Templates

* Fixed typos in the pre-installed VMware Cloud Director template, and the `internalNetworkCIDR` parameter has been moved to input parameters of the template.

### Chore

* The built-in console has been updated to the current version 1.34.4

## v1.10.1

### New Features

#### Interface

* Added display of "Allow management in cluster" changes in audits.
* Added "Processing Options" item in the cluster menu, which allows opening cluster settings with one click.

#### Cluster Manager

* Added the ability to use the `sudoPassword` field in cluster connection settings (also, for `sudoPassword` to work from the dhctl side, version 1.68.3 or higher is required).

### Bug Fixes

#### Interface

* Fixed rendering of templates with a custom container registry certificate, which led to the `invalid cert in CA PEM` installation error.
* Fixed inability to save a form until all required fields are filled (restored previous behavior, allowing saving and highlighting validation errors if any).
* Fixed incorrect display of resource group diffs after a request to the renderer.
* No audit record was created when updating the management mode for a cluster resource group.
* Fixed a 403 error when trying to open API documentation.
* Added output of missing data in audits for correct display of changes.

#### Module

* Fixed discovered vulnerabilities in module images.
* Removed redis-exporter component for collecting Redis metrics due to a large number of unresolved vulnerabilities.

#### Cluster Manager

* Fixed errors occurring when there is no pre-launched dhctl instance in the cluster, and the dhctl image fails to download within 1 minute (timeout increased to 5 minutes). This error often occurred in user configurations with enabled `devBranch`.

## v1.10.0

### New Features

#### Interface

* A mode for synchronizing Kubernetes resources in the cluster has been introduced. You can turn resource synchronization on or off in each group of resources. This mode can be specified in the cluster template.
* Resources can be "released" for management within the cluster without deletion:
If a resource group is not controlled, the agent won't delete resources that have vanished in it but will label them with `agent.commander.deckhouse.io/is-orphan`.
If the group is controlled, resources are removed as before.
* Cluster processing control by the cluster manager has been redesigned:
In the cluster settings (Configuration -> Infrastructure operations -> Settings), you can select the "Change application mode." If "Manual" is selected, any changes (converge) to the cluster will require user confirmation of the operation. You will need to click a button to start the operation.
"Auto" works as before — the user will be asked to confirm changes only if they are destructive.
* Deleting a workspace is now possible.
But there is a condition: you first need to get rid of the clusters. They can be deleted, detached, or moved to another workspace. Other data — cluster templates and inventory — are deleted **irreversibly**.
* In the archive, all lists are now sorted by deletion date by default, starting with the most recent.

#### Server

* All API server logs are now output in structured JSON format. It has become more convenient to parse with collection systems and analyze logs.
* Audit logs now include information about specific changes (field audited_changes) and the user's IP address.
For example, logs will reflect previous and current values: template version, input parameters, cluster synchronization interval, and other changes that do not contain sensitive data.

#### Templates

* A new pre-installed template for Huawei Cloud (cloud.ru) has been added.

### Bug Fixes

#### Documentation

* Outdated information about the fixed version of dhctl has been removed.
* A shorter and clearer explanation of cluster structures has been added.
* Network accessibility requirements for the commander and clusters are specified in text in addition to what's shown in the diagram.
* It is explicitly stated that the synchronization interval management is handled by the cluster manager, but it does not affect the agent's work.
* Removed the migration instruction from deckhouse-commander module (1.4) to commander module (1.5).
* Fixed the link to the input parameters schema (anchor within the same page).

