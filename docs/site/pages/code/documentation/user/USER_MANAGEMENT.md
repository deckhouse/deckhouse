---
title: "User, access and change management"
permalink: en/code/documentation/user/user-management.html
---

Security is one of the key aspects of Deckhouse Code. The tool provides built-in mechanisms for source code protection, access control, and audit logging.

Deckhouse Code implements a multi-level access control model that ensures the security of both individual repositories and the entire infrastructure.

Features:

- **Role-based access control (RBAC)** — predefined roles are supported: Guest, Reporter, Developer, Maintainer, Owner.
- **Protected branches** — direct changes are prohibited; modifications are allowed only through merge requests.
- **Protected tags** — control over tag creation and modification.
- **Authentication via external providers** — support for SAML, LDAP, and OIDC.
- **Group-wide access policies** — centralized security management for all projects within a group.

## Audit and activity logging

Deckhouse Code records user and administrator actions for security auditing and analysis.

Tracked events:

- Changes in project or group settings.
- Branch and tag creation, deletion, and modification.
- User permission changes.
- SSH key addition and removal.
- Successful and failed login attempts.

Logs are available via the web interface or can be exported to external monitoring systems.

## Two-factor authentication (2FA) support

To enhance account security, two-factor authentication can be enabled.

Supported methods:

- Authenticator apps (e.g., Google Authenticator, Authy).
- Hardware security keys (U2F, WebAuthn).

The administrator can enforce 2FA for all project users.

## SSH and HTTPS access control

Deckhouse Code allows secure connections to repositories using the following methods:

- **SSH keys** — a secure method of authentication without using a password.
- **HTTPS + Personal Access Tokens** — an alternative method using access tokens.

Additional configuration options:

- Restrict allowed protocols (HTTPS-only or SSH-only).
- Disable password login — only keys or tokens are allowed.
