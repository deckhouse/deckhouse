---
title: "Protected branches"
menuTitle: Protected branches
force_searchable: true
description: Protected branches
permalink: en/code/documentation/user/protected-branches.html
lang: en
weight: 50
---

Protected branches restrict changes to important branches of a repository: only those who are explicitly allowed can push to or merge into such a branch. Protection is typically used for `main`, `master`, release, and stable branches — so that changes reach them only through a merge request and code review.

The default branch of a project is protected automatically when the project is created.

## What branch protection does

The following restrictions apply to a protected branch:

- Direct pushes (`git push`) are rejected for everyone except those listed in "Allowed to push and merge". Other members contribute changes through merge requests.
- Force pushes (`git push --force`) are rejected for everyone unless the "Allow force push" setting is enabled in the branch rule.
- The branch cannot be deleted with Git commands. It can only be deleted through the web interface by users with the `Maintainer` role or higher.

## Default branch protection

When a project is created, its default branch becomes protected automatically. The initial protection level is controlled by the "Initial default branch protection" setting:

- **At the instance level**: "Admin area" → "Settings" → "Repository", the "Default branch" section. Available to instance administrators.
- **At the group level**: on the group page, go to "Settings" → "Repository", the "Default branch" section. Available to users with the `Owner` role in the group.

The group setting takes precedence over the instance setting: if it is not set for the group, the instance setting applies. Projects in a user's personal namespace always use the instance setting.

By default, full protection is applied: push and merge are allowed only for members with the `Maintainer` role or higher, and force pushes are rejected.

## Branch rules

Branch protection is configured through branch rules: open the project page and go to "Settings" → "Repository" → "Branch rules". Configuration is available to users with the `Maintainer` and `Owner` roles.

To create a rule:

1. Click "Add branch rule".
1. Select "Branch name or pattern".
1. Enter the name of a specific branch (for example, `main`) or a pattern with the `*` wildcard (for example, `release/*`). A rule with a pattern protects all matching branches, including ones created later.
1. Click "Create branch rule".

To open the settings of an existing rule, click "View details" next to it in the list.

## Configuring access to a protected branch

On the rule page, the "Protect branch" section contains two access lists:

- **"Allowed to merge"** — who can merge merge requests into this branch.
- **"Allowed to push and merge"** — who can push changes directly to the branch (`git push`) and also merge.

Click "Edit allowed to merge" or "Edit allowed to push and merge" to change the corresponding list. Each list can combine roles, individual users, and groups; the "Allowed to push and merge" list additionally supports deploy keys. Access is granted to anyone who matches at least one of the selected entries.

Available options:

| List entry | Who gets access |
|---|---|
| "Maintainers" | Project members with the `Maintainer` role or higher. |
| "Developers and Maintainers" | Project members with the `Developer` role or higher. |
| "No one" | Nobody. Role-based access is fully disabled. |
| "Administrators" | Instance administrators. |
| "Users" | The listed project members with write access to the repository (the `Developer` role or higher). See [Access for individual users and groups](#access-for-individual-users-and-groups). |
| "Groups" | Direct members of the listed groups with the `Developer` role or higher in the group. The group must be invited to the project with at least the `Developer` role. See [Access for individual users and groups](#access-for-individual-users-and-groups). |
| "Deploy keys" | The owner of the selected deploy key, including pushes made with the key itself. The key must be enabled in the project with write access, and its owner must be a project member. Only available in the "Allowed to push and merge" list. |

## Access for individual users and groups

Besides roles, access to a protected branch can be granted selectively — to specific users and groups. For example, you can close the branch to everyone (by selecting "No one") and allow merging only for the release manager, without raising their role in the project.

### Users

In the "Users" selector you can pick project members with write access to the repository — the `Developer` role or higher (including members inherited from the parent group).

Adding a user to the list does not elevate their permissions in the project: a member without the `Developer` role or higher cannot push to the protected branch even if they are listed. The purpose of these lists is to narrow down the set of people whose role already grants them write access. For example, select "No one" for roles and list specific users — then only they can work with this branch.

### Groups

The "Groups" selector only shows groups invited to the project with the `Developer` role or higher (group invitations are managed in the project's "Manage" → "Members" section). Ancestor groups of the project and groups invited with a lower role cannot be selected.

Only direct members of the invited group with the `Developer` role or higher in that group get access through it. Members of nested subgroups do not get access.

### When access stops working

Permissions are checked at the moment of each operation, so membership changes take effect immediately:

- if a user is no longer a project member, their access to the protected branch stops working;
- if the group's invitation to the project is revoked or its role is downgraded below `Developer`, access for the group's members stops working.

The entry remains in the access list and becomes effective again if the membership or invitation is restored. To revoke access permanently, remove the user or group from the list in the branch rule.

## Allow force push

The "Allow force push" toggle on the rule page allows force pushes (`git push --force`) to the protected branch. Who exactly can force push is determined by the "Allowed to push and merge" list. The toggle is off by default, and force pushes are rejected for everyone, including maintainers and administrators.

## Notes and limitations

- Access granted on a protected branch does not change the user's role in the project and does not grant any other permissions — it only affects push and merge operations on branches matched by the rule.
- A rule with a pattern (for example, `release/*`) applies to all existing and future branches whose names match the pattern.
- Selecting "No one" disables role-based access but does not clear the lists of users, groups, and deploy keys — those listed still have access.
- Only users with the `Maintainer` role or higher can configure branch rules.
