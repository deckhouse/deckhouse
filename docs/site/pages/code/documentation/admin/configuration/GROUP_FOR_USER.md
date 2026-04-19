---
title: "Create a group for users"
permalink: en/code/documentation/admin/configuration/group.html
---

1. Log in as an administrator:
   - Go to your Deckhouse Code installation using the public URL.
   - Use administrator credentials.
   - During initial setup, use the `root` user. The password is available as a secret in your cluster namespace:

     ```console
     secrets/initial-root-token
     ```

1. Create a group:
   - Go to the "Admin" section — the bottom button in the left sidebar.
   - Navigate to "Groups" → "New Group".
   - Specify the group name (e.g., "Development Team").
   - Set the visibility level:
     - "Private" — visible only to group members.
     - "Public" — visible to everyone without exception.
     - "Internal" — visible to all registered users.

1. Invite users:
   - Open the created group.
   - Go to the "Manage Access" section.
   - Click "Invite members", enter the user's email or username, and assign a role (e.g., "Developer").

   > **Important**. When using SSO integrations, the access setup process may differ.

1. Assign roles to group members.

Depending on the assigned role, users have different levels of access:

**Guest**:

- Group: Can view public items. No access to confidential data.
- Project: Can view issues in public repositories, but cannot comment or manage them.

**Reporter**:

- Group: Can view all information, including issues and public projects.
- Project: Can read code, CI/CD pipelines, and reports without making changes.

**Developer**:

- Group: Can create projects and modify repositories.
- Project: Has access to branches, commits, merge requests, CI/CD, and development tasks.

**Maintainer**:

- Group: Full control over projects and group members.
- Project: Can manage project settings, branches, tags, and merge requests.

**Owner**:

- Group: Full control, including deletion and role management.
- Project: Has group-level access with full project management rights.

1. Go to the desired group.
1. Select "Manage" → "Members".
1. Assign the appropriate role to each invited user.
