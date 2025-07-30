---
title: "Configuring MergeRequestApprovals"
permalink: en/code/documentation/admin/configuration/mr-approvals.html
---

At the installation level, Deckhouse Code is already configured, but enabling this functionality for a specific project requires additional steps.

MRA is an optional feature that allows you to:

- Define rules that determine which merge requests must be approved before being merged into a given branch.
- Use the CODEOWNERS feature to automatically identify who must approve a merge request when specific files or groups of files are modified (based on type, pattern, etc.).

The instructions below outline the steps for implementing basic merge request approval rules for the `main` branch:

1. Open the project you want to configure.
1. Select "Settings" → "Merge Requests" from the left navigation panel. These actions are available either to the system administrator or to users who have the appropriate role to change project settings.
1. Enable the "All discussions must be resolved" option, as shown in the screenshot below.

1. Ensure that the `deckhouse_sa` account has `Owner` access to the project or group. If not, follow the instructions in the second section of this document.
1. Create a file named `approval_rules.yaml` in the root of the project's default branch (by default, this is the `main` branch).
1. Populate the file with configuration.

Example configuration:

```yaml
policies:
  - name: test_policy_group
    approvers:
      groups:
        - test_group
    count: 1
    includeAuthor: true

  - name: test_policy_user
    approvers:
      users:
        - user1
    count: 1
    includeAuthor: true

rules:
  - name: any
    policies:
      or: 
        - policy: test_policy_user
        - policy: test_policy_group

branches:
  - names: 
      - 'main'
    rule: any
```

In the provided configuration example, a merge request to the `main` branch will be allowed only after receiving approval from either the user `user1` or any member of the `test_group` group.
