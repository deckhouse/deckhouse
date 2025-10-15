---
title: "Secret management"
permalink: en/user/web/stronghold.html
---

The web interface for managing secrets (Stronghold web interface) is designed for managing secret mechanisms, authentication, and access control in a cluster. Operation is provided by the `stronghold` module.

It allows you to:

- View and configure secrets engines, add and edit secrets.
- Manage authentication methods, user groups, and entities.
- Control permissions, leases, and security policies.

## Accessing the stronghold web UI

To open the web UI, enter `stronghold.<CLUSTER_NAME_TEMPLATE>` in your browser's address bar,
where `<CLUSTER_NAME_TEMPLATE>` is the DNS name template of the cluster
defined in the global `modules.publicDomainTemplate` parameter.

1. On your first login, enter the user credentials.
1. After successful authentication, the main `stronghold` page will open:

   ![Stronghold module web UI](../../images/stronghold/web-stronghold.png)

## Managing secrets engines

### Viewing a secrets engine

To view a secrets engine:

1. Click on its name.
1. The following tabs will be displayed:
   - "Secrets" with a list of secrets (roles, keys, etc.).
   - "Configuration" with engine configuration.
   - A button to add a new secret.

![Secrets engine](../../images/stronghold/mechanism-of-secrets.png)

To view the configuration of an engine, go to the "Configuration" tab.
The content of this tab depends on the selected engine.

![Configuration of an engine](../../images/stronghold/configuration-of-secrets.png)

To view secret information and versions (for example, for a "key-value" engine):

1. Click the secret's name in the list.
1. You will see the following tabs:
   - "Secret" with general information about the secret and version history.
   - "Metadata" with metadata of the secret.

![View secret information](../../images/stronghold/secrets-information.png)

The "Secret" tab includes:

- A toggle to switch to JSON view.
- Buttons for:
  - Deleting the secret.
  - Copying the secret.
  - Selecting a version (if available).
  - Adding a new version.

![Secret tab](../../images/stronghold/secret.png)

To view metadata, switch to the "Metadata" tab.

![Metadata](../../images/stronghold/metadata.png)

To add a new secret, follow these steps:

1. Click the button for adding a secret (button may vary depending on the engine: role, key, etc.).

   ![Add a new secret](../../images/stronghold/add-secret.png)

1. In the form that opens, provide the required parameters (fields vary by engine).
   For example, the form in a "Cubbyhole" engine includes the following fields:
   - A JSON view and edit toggle.
   - A path field.
   - A key field.
   - A value field.
   - A button to add additional key-value pairs under the same path.
1. Click **Save** to finish.

    ![Add a new secret](../../images/stronghold/add-form.png)

To add a new secrets engine:

1. Click **Enable new engine**.

   ![Enable new engine](../../images/stronghold/mechanism.png)

1. Choose an engine type and click **Next**.
1. In the setup window, provide:
   - Basic engine configuration (depends on type)
   - Optional method settings (click the header to expand)
1. Click **Enable Engine** to save the engine.
   Alternatively, click **Back** to cancel and return.

   ![Basic engine configuration](../../images/stronghold/mechanism-settings.png)

## Managing access to data and stronghold features

Access control is managed in the "Access" section.

To open it, click **Access** in on the main page of `stronghold` web UI.
The left pane contains a navigation menu with a link to return to the main page.
The central pane displays content for the selected subsection (by default it's "Authentication Methods").

### Working with authentication methods

This subsection opens by default when entering the "Access" section.
You can also click "Authentication Methods" in the left menu.

![Work with method](../../images/stronghold/work-method.png)

This subsection shows:

- A list of authentication methods configured in the cluster.
- Filters for the list.
- A button to enable new method.

Available actions for each method:

- View configuration.
- Edit configuration.
- Delete method.

To view method details, click its name or select "View configuration".
A configuration tab will appear with a button to configure the method.

For example, for the `oidc_deckhouse` method, the "Configuration" tab and a "Configure" button are shown.

To add a new authentication method:

1. Click **Enable new method** in the "Authentication Methods" section.
1. Choose a method from the list and click **Next**.
1. In the opened window, provide:
   - Path
   - Optional method options (click the header to expand)
1. Click **Enable Method** to finish.
   Alternatively, Click **Back** to cancel and return.

### User groups

To manage user groups, click **Groups** in the side menu.

This subsection provides:

- A list of user groups in the cluster.
- Filters for the list.
- A button to add a new group.

Available actions for listed groups:

- View group details.
- Edit group settings.
- Delete a group.

To view group details, click its name or select **Details**.

To add a new user group:

1. Click **Create group** in the "Groups" section.
1. Fill out the group creation form.
1. Click **Create** to save.
   Alternatively, click **Back** to cancel and return.

### Entities and aliases

Entities in `stronghold` are logical representations of users or applications.
They allow linking multiple authentication methods under a single identity.

To manage entities and aliases, select **Entities** from the side menu of the "Access" section.

The center panel has two tabs:

- **Entities** with a list of entities.
- **Aliases** with a list of aliases.

Also available:

- Filters.
- **Merge entities** button to combine identities.
- **Create entity** button.

Available actions:

- For entities:
  - View details.

    ![Info entities](../../images/stronghold/info-entities.png)

  - Create an entity.

    ![Create entity](../../images/stronghold/create-entity.png)

  - Delete an entity.

    ![Merge entities](../../images/stronghold/merge-entities.png)
  
- For Aliases:
  - View details.

    ![Info alias](../../images/stronghold/info-alias.png)

  - Edit an alias.

    ![Create alias](../../images/stronghold/create-alias.png)

  - Create an alias.
  - Delete an alias.

### Managing temporary access to secrets and resources (leases)

To manage leases (temporary access to secrets and resources), click **Leases**.
A search box will open that lets you find lease details by ID.

![Leases](../../images/stronghold/leases.png)

## Managing access control policies

To manage access control policies in `stronghold`, click **Policies** from the "Access" menu.
The left pane shows navigation menu.
The central pane contains a list of policies, a search filter, and a button to add a new policy.

For actions with a policy, use the three-dot menu at the end of the row with the policy's name.

Available actions for listed policies:

- **View policy details**. To view information about a policy, click its name or select "Details".
  The policy details window shows information in HCL format, a button to download the data, and a button to edit the policy.

  ![Info policies](../../images/stronghold/info-policies.png)

- **Edit policy**. To add a policy, click **Create ACL policy** and enter its name and HCL configuration.

  ![Create policies](../../images/stronghold/edit-policies.png)

- **Delete policy**.

## Using additional tools

The "Tools" section provides access to extra tools in `stronghold`.
The left pane displays tool navigation.
The center shows fields for the selected tool.

Available tools:

- **Wrap**: Creates a wrapping token for secure transfer of confidential data and secrets.
  This token can be transferred to another user or an application for subsequent unwrapping and accessing the data.

  ![Wrap](../../images/stronghold/wrap.png)

- **Lookup**: Displays details about tokens, secrets, leases, and other objects in `stronghold`.
  It can be used to view metadata, expiration dates, access policies, and other details associated with objects.

  ![Lookup](../../images/stronghold/lookup.png)

- **Unwrap**: Unwraps a wrapping token to access the wrapped data.

  ![Unwrap](../../images/stronghold/unwrap.png)

- **Rewrap**: Reissues a wrapping token based on an existing one.
  This lets you extend the token's TTL or modify its parameters without disclosing protected data.

  ![Rewrap](../../images/stronghold/rewrap.png)

- **Random**: Generates cryptographically secure random data to create unique IDs, tokens, passwords
  and other data that requires a higher grade of randomness and security.

  ![Random](../../images/stronghold/random.png)

- **Hash**: Generates hashes for different data. Supports multiple caching algorithms.

  ![Hash](../../images/stronghold/hash.png)

- **API Explorer**: Used for interacting with `stronghold` API via GUI.

  ![API](../../images/stronghold/api.png)

## Monitoring stronghold Raft cluster

Use the "Raft Storage" section to monitor the `stronghold` Raft cluster.
The left menu contains a navigation menu; the central pane shows the cluster leader, nodes,
and a **Snapshots** button for backing up or restoring Raft data from a snapshot.

![Raft](../../images/stronghold/raft.png)

## Monitoring activity and load

To monitor activity and assess `stronghold` workload, go to the "Client Count" section.
The left pane contains a navigation menu.

The cental pane contains the following two tabs:

- **Dashboard** with a number of unique clients for the current month and buttons to change the period.

  ![Monitoring](../../images/stronghold/monitoring.png)

- **Configuration** that allows editing of metric collection settings (use the **Edit configuration** button for that).

  ![Congfiguration](../../images/stronghold/configuration.png)

## Sealing and unsealing the secrets storage

The "Seal Stronghold" section provides means to seal or unseal the secrets storage.
The left pane contains a navigation menu.
The cental pane contains a button for sealing or unsealing the secret storage (depending on its current state).

When the storage is sealed, `stronghold` can't read or write secrets.

![Sealing and unsealing the secrets storage](../../images/stronghold/sealing.png)

## Using the stronghold CLI

The stronghold CLI tool can perform operations related to secrets, policies, users, and more.
You can launch it from any interface section via the CLI button in the upper-left corner. Click again to close it.

![Stronghold](../../images/stronghold/stronghold-cli.png)
