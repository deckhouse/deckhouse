---
title: "Group Wiki"
menuTitle: Group Wiki
force_searchable: true
description: Group Wiki
permalink: en/code/documentation/user/group-wiki.html
lang: en
weight: 50
---

## Group Wiki

> Documentation for project Wikis can be found  
> in the GitLab documentation.

### Enabling Wiki at the Group Level

On the group page, go to `Settings → General → Permissions and group features → Wiki access level`.

Available options:
- **Enabled**. For public groups, everyone can access the wiki. For internal groups, only authenticated users can access the wiki.
- **Private**. Only group members can access the wiki.
- **Disabled**. The wiki isn’t accessible, and cannot be downloaded.

Default value — **Enabled**.

### Accessing the Wiki

To open the group Wiki, go to the group page and select `Plan → Wiki`.

### Roles and Wiki Access

Access level is determined by the user's membership role in the group:

| Role                             | Actions                                                                                                     |
|----------------------------------|------------------------------------------------------------------------------------------------------------|
| **Guest**                        | Read the Wiki                                                                                              |
| **Reporter**                     | Read, download Wiki code                                                                                   |
| **Developer**                    | Create Wiki pages                                                                                          |
| **Maintainer**                   | Administer the Wiki                                                                                        |
| **Planner**                      | Administer the Wiki                                                                                        |
| **Anonymous / external user**    | Access if the group is public or internal and the user is not external — download Wiki code only           |

### Features

#### 📁 Structure and Nesting

- Create hierarchical page structures using `/` (slash) in page names.  
  Example:

  ```text
  devops/ci-pipelines
  devops/kubernetes
  product/design
  ```

  This creates folder-like navigation and helps organize content.

---

#### 📝 Markdown, Rich Text, Attachments, and Diagrams

- **Markdown (GitLab Flavored)**:
  - Mentions (`@username`), issue/MR references (`#123`, `!456`), tasks (`- [ ]`), tables, code blocks.
  
- **Rich Text Editor**:
  - WYSIWYG editing for users who prefer formatting without Markdown.
  - Supports Mermaid and Draw.io integration as is.

- **Attachments**:
  - Upload and embed images, PDFs, and other files.
  - Stored inside the Wiki’s Git repository.

---

#### 🕘 Version History

- Full edit history for every page:
  - Who made changes.
  - When they were made.
  
- View diffs, roll back changes, and track all edits.

---

#### 💬 Discussions and Comments

- Comments and discussions on pages.

---

#### ⬇️ Git Access

- Each Wiki is a dedicated Git repository.
- Clone via SSH or HTTPS:

  ```bash
  git clone git@code.example.com:groupname/wiki.git
  ```

- Full access to `.md` files, branches, history — ideal for local editing, backup, or automation.

---

#### 📄 Export to PDF

- Export any Wiki page as a PDF via the web interface.
- Useful for offline access, sharing, or printing.

---

#### 🧩 Page Templates

- Reusable templates stored under the `templates/` directory.
- Apply templates when creating or editing pages to standardize content.

---

#### 🗂 Customizable Sidebar

- Sidebar shows pages as a nested tree.
- Fully customizable via a special `_sidebar` page.
- Add links, organize sections, and improve navigation.
