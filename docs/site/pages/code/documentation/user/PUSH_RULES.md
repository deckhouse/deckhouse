---
title: "Push Rules"
menuTitle: Push Rules
force_searchable: true
description: Push Rules
permalink: en/code/documentation/user/push-rules.html
lang: en
weight: 50
---

Code provides a set of push rules to enforce specific policies on commits and Git pushes. These rules help maintain repository integrity, improve code quality, and ensure compliance with your organization’s security standards.

### Available rules

- ***Verify committer email***  

  Committer email must be verified in the Code profile.

- ***Prevent tag deletion***  

  Tags cannot be deleted via git push. Deletion through the Web UI is available.  

- ***Member check***

  Commit author must be a Code member.

- ***Prevent secret leaks***  

  Any attempt to commit files that match the following patterns will be rejected:
  
  - `aws/credentials`,
  - `ssh/personal_rsa`, `ssh/personal_dsa`, `ssh/personal_ed25519`, `ssh/personal_ecdsa`, `ssh/personal_ecdsa_sk`, `ssh/personal_ed25519_sk`,
  - `ssh/server_rsa`, `ssh/server_dsa`, `ssh/server_ed25519`, `ssh/server_ecdsa`, `ssh/server_ecdsa_sk`, `ssh/server_ed25519_sk`,
  - `id_rsa`, `id_dsa`, `id_ed25519`, `id_ecdsa`, `id_ecdsa_sk`, `id_ed25519_sk`,
  - files with extensions `.pem` or `.key`,
  - files `.history` or `_history`,
  - files with extensions `.keystore` or `.jks`,
  - `keystore.p12`, `keystore.pfx`.

- ***Require GPG signature***  

  Only GPG-signed commits are allowed. This rule also rejects changes via the Web UI if GPG signing is not configured for the UI.  

- ***Require DCO (Developer Certificate of Origin)***  

  Commits must include a `Signed-off-by` line to indicate agreement with the DCO. With this rule enabled, the **Revert** option for merge requests in the Web UI is unavailable, since the auto-generated commit message does not contain a sign-off.  

- ***Verify committer name***  

  Committer’s name must match the Code profile name.  

- ***Commit message regex***  

  Commits must match the specified regex pattern. Leaving the field empty disables the rule.  

- ***Author email regex***  

  Author email must match the specified regex pattern. Leaving the field empty disables the rule.  

- ***File name regex***  

  File names must match the specified regex pattern. Leaving the field empty disables the rule.  

- ***Branch name regex***  

  Branch names must match the specified regex pattern. Leaving the field empty disables the rule.  

- ***Negative commit message regex***  

  Commits will be rejected if their message matches the specified regex pattern. Leaving the field empty disables the rule.  

- ***Max file size (MB)***  

  Files larger than the specified size (in megabytes) will be rejected. To disable this rule, set the value to `0`.  

## Enabling push rules

To configure push rules for a project, open the project page and go to "Settings → Repository → Push Rules".  

Only users with the `Maintainer*` or `Owner` role can configure push rules.  

## Configuring push rules at group or instance level

To configure push rules at the group level, open the group page and go to "Settings → Repository → Push Rules". This option is available to users with the `Owner` role.  

To configure push rules at the instance level, go to "Admin → Settings → Repository → Push Rules". This option is available only to administrators.  

When a rule is changed at the instance level, the new values are automatically applied to all groups and projects.  

When a rule is changed at the group level, the new values are automatically applied to all subgroups and their projects.  

Rules defined at the group or instance level are used as default settings for projects. When a project is created, it inherits the rules from its parent group or from the instance (if created in a personal namespace).  

## Restricting rule overrides

You can prevent overriding of rules in child projects or groups. To do this, uncheck the option "Allow override at group/project level". Such a rule cannot be modified in child groups or projects. At the instance level, this applies to all groups and projects.  

**Examples:**  

1. If you enable the *"Verify committer email"* rule at the group level and disallow overriding, this rule will be enabled in all projects of that group and its subgroups, with no option to disable it.  

1. If you disable the same rule at the group level and disallow overriding, it will be disabled in all projects of that group and its subgroups, with no option to enable it.  

1. If overriding is allowed, the rule will still be automatically updated when it changes at the parent level, but child groups and projects will be able to modify it afterwards.  

The inheritance hierarchy is as follows:  
**Instance → Group → Subgroup → Project**
