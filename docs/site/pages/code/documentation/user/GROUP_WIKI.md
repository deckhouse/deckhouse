---
title: "Group Wiki"
menuTitle: Group Wiki
force_searchable: true
description: Group Wiki
permalink: en/code/documentation/user/group-wiki.html
lang: en
weight: 50
---

## Enabling wiki at the group level

To enable or configure access to the Wiki, open the desired group page and go to: “Settings” → “General” → “Permissions and group features” → “Wiki access level”.

Available options:

- Enabled — the Wiki is available to everyone (for public groups) or only to authenticated users (for internal groups).
- Private — the Wiki is accessible only to group members.
- Disabled — the Wiki is completely disabled and cannot be accessed.

Default value — Enabled.

## Accessing the wiki

To open the group Wiki, go to the group page and select “Plan” → “Wiki”.

## Roles and wiki

Access level is determined by the user's group membership:

| Role                              | Actions                                                                                                           |
|-----------------------------------|------------------------------------------------------------------------------------------------------------------|
| **Guest**                         | View the Wiki                                                                                                   |
| **Reporter**                      | View the Wiki, download code                                                                                    |
| **Developer**                     | Create wiki pages                                                                                                |
| **Maintainer**                    | Administer the Wiki                                                                                             |
| **Planner**                       | Administer the Wiki                                                                                             |
| **Anonymous / external user**     | Access granted if the group is public or internal, and the user is not external — only code download allowed    |

## Features

1. Structure and nesting — creates a tree navigation and helps organize content:

   - Create a hierarchical page structure using the `/` symbol in names. Example:

     ```console
     devops/ci-pipelines
     devops/kubernetes
     product/design
     ```

1. Markdown, rich text, attachments, and diagrams:

   - Markdown (GitLab Flavored):
     - Mentions (`@username`), references to issues/MRs (`#123`, `!456`), checklists (`- [ ]`), tables, code blocks.
  
   - Rich text editor:
     - WYSIWYG editor for users who prefer visual formatting.
     - Built-in support for Mermaid and Draw.io.

   - Attachments:
     - Upload and embed images, PDFs, and other files.
     - Stored in the Wiki's Git repository.

1. Change history:

   - Complete history of changes for each page:
     - Who made the changes.
     - When the changes were made.
  
   - View diffs, revert changes, and track revisions.

1. Discussions and comments on pages.

1. Git access:

   - Each Wiki is a separate Git repository.
   - Clone via SSH or HTTPS:

     ```bash
     git clone git@code.example.com:groupname/wiki.git
     ```

   - Full access to `.md` files, branches, and history for local editing, backups, or automation.

1. PDF export:

   - Export any Wiki page to PDF via the web interface.
   - Useful for offline access, sharing, or printing.

1. Page templates:

   - Reusable templates stored in the `templates/` directory.
   - Applied when creating or editing pages to standardize content.

1. Customizable sidebar:

   - The sidebar displays pages in a nested tree structure.
   - Fully customizable via the special `_sidebar` page.
   - You can add links, sections, and improve navigation.
