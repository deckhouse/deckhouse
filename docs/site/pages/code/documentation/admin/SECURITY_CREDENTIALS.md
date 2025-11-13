---
title: "Security credentials"
menuTitle: Security credentials
force_searchable: true
description: Security credentials
permalink: en/code/documentation/admin/security-credentials.html
lang: en
weight: 50
---

The Security credentials section provides a comprehensive overview of all tokens (personal, group, project) and keys (SSH, GPG) on the Code platform.
Administrators can switch tabs of different credential types, apply filters and sorting options, and delete or revoke any credential to limit user access and enhance security.

## Purpose

The security credentials section is used for:

- Providing a complete overview of all security credentials (ownership, scope, creation/expiration/revocation dates, etc.).
- Filtering and sorting credentials.
- Managing the credential lifecycle: revocation, deletion.

## Accessing security credentials via UI

To access security credentials, switch to the admin mode and select "Security credentials" in the sidebar. This opens the security credentials table.
The section consists of 4 tabs:

- **Personal Access Tokens**: List of tokens, which belong to real users.
- **SSH Keys**: List of SSH keys, which belong to real users.
- **GPG Keys**: List of GPG keys, which belong to real users.
- **Group/Project Access Keys**: List of tokens, which belong to top-level groups or projects.

![Security credentials table](/images/code/security_credentials_table_en.png)

### Available actions

The set of actions that can be performed on a credential depends on its type. The table shows the relationship between actions and the credential type.

| Action\Credential type | Personal Access Token | SSH Key | GPG Key | Group/Project Access Token |
|------------------------|-----------------------|---------|---------|----------------------------|
| Removal                | Yes                   | Yes     | Yes     | Yes                        |
| Revocation             | Yes                   | No      | No      | No                         |
