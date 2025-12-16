---
title: "CODEOWNERS"
menuTitle: CODEOWNERS
force_searchable: true
description: CODEOWNERS
permalink: en/code/documentation/user/code-owners.html
weight: 50
---

The **CODEOWNERS** feature allows you to determine individuals responsible for specific parts of the repository. Users and groups can be determined as responsible parties. Changes affecting files specified in `CODEOWNERS` must be approved by the code owners.

Using CODEOWNERS helps:

- require approval from domain experts for important directories
- simplify the process of finding those responsible for a specific section of code.

CODEOWNERS complements the [approval rules](/products/code/documentation/user/approval-rules.html) mechanism, but does not replace it. Unlike approval rules, which are set manually in the UI, CODEOWNERS works through the `CODEOWNERS` file in the repository.

## How CODEOWNERS works

`CODEOWNERS` is a text file in the repository.
Deckhouse Code uses it to determine the owners of files involved in a merge request.

It is searched in three places (in order of priority):

1. `./CODEOWNERS`.
1. `./docs/CODEOWNERS`.
1. `./.gitlab/CODEOWNERS`.

**Only the first file found** is used.

If there are several files in the project and you are currently viewing a non-priority file, you will see a corresponding message.

![Non-priority file](/images/code/not_primary_code_owners_en.png)

### Validating the CODEOWNERS file

Validation helps you find errors in the file during editing.

If the file is valid, the following message is displayed:

![Valid file](/images/code/code_owners_valid_en.png)

If the file is invalid, the problematic lines and the type of error will be displayed:

![Invalid file](/images/code/code_owners_invalid_en.png)

There are four types of validation errors

- *Owners either do not have project access or their access level not enough*: At least one of the owners does not have sufficient rights in the project.
- *Spaces in path*: If the path contains spaces, they must be escaped with the `\` character.
- *Zero owners*: No owners are specified.
- *Not a closed section*: The closing section character `]` is missing.

### Where CODEOWNERS is used

If the MR modifying the file falls under the CODEOWNERS rules, approval must be obtained from the owners.

### CODEOWNERS format

Rule string:

```console
<path or pattern> <list of owners>
```

Owners:

- `@user`
- `@group`
- `@group/subgroup`
- roles: `@@developer`, `@@maintainer`, `@@owner`.

### Examples

```console
# Owner of all files
* @default-team

# README.md only in the root directory
/README.md @docs-team

# All Ruby files
*.rb @backend-team

# The entire config directory
/config/ @devops

# README.md anywhere
README.md @docs
```

## Path patterns

Deckhouse Code uses wildcards:

- `*`: Any characters except `/`.
- `**`: Matches multiple directory levels.
- `/dir/`: Only the specified directory.
- `*.md`: All Markdown files.
- `/config/**/*.rb`: All Ruby files inside `config/*` at all levels.

### Pattern examples

```console
/docs/*       @docs-one      # one level
/docs/**      @docs-two      # recursively
/app/**/*.rb  @ruby-team
```

## Секции CODEOWNERS

In the `CODEOWNERS` file, sections are named areas that are analyzed separately and always applied. Until you define a section, Deckhouse Code treats the entire `CODEOWNERS` file as a single section.

Features of using `CODEOWNERS`:

- Deckhouse Code treats entries without sections, including rules defined before the first section, as a separate, unnamed section.
- Each section processes its rules separately.
- If the file path matches multiple entries within a single section, only the last matching entry in that section is used.
- If the file path matches entries in multiple sections, the last matching entry in each section is used.

For example, in a `CODEOWNERS` file with the following sections defining owners for `README`:

```console
* @root

[README Owners]
README.md @user1 @user2
internal/README.md @user4

[README other owners]
README.md @user3
```

Owners for `README.md` in the root directory:

- `@root`: From the unnamed section.
- `@user1` and `@user2`: From the `[README Owners]` section.
- `@user3`: From the `[README other owners]` section.

Owners for `internal/README.md`:

- `@root`: From the unnamed section.
- `@user4`: From the `[README Owners]` section. (Both `README.md` and `internal/README.md` match the section rules, but only the last matching entry in this section is used).
- `@user3`: From the `[README other owners]` section.

In the merge request widget, each code owner is displayed under their own label.

![Codeowners example](/images/code/code_owners_en_example.png)

### Detailed example: complex combination of sections

Below is an example of a real industrial file showing:

- sections with different numbers of approvals
- redefining owners
- optional sections
- exceptions
- inheritance of sections with identical names.

```console
# Global settings
* @fallback-team
!*.lock                    # lock files do not require approval
!**/generated/**           # automatic generation

[Backend][2] @backend-core
# Requires 2 approvals from backend-core for any Ruby code
app/**/*.rb

# But for critical models, we define a different order
app/models/**/*.rb @backend-core @security-team

# But this model is an exception; the owner of another one
app/models/legacy/**/*.rb @migration-team

[Backend]
# Section update: Backend now also includes SQL
db/**/*.sql @db-team

[Ruby Optional]
# Additional lighting for owners, approvals are NOT required
^[Ruby Optional]
*.rb @ruby-advisors

[Frontend][3]
# The UI requires 3 approvals.
*.vue @frontend-team
*.js  @frontend-team

# redefining: specific file → specific person
frontend/critical_entry.vue @frontend-lead

[Docs]
*.md @technical-writers

[Docs]
# redefinition: README always belongs to docs-lead
README.md @docs-lead
```

Features of the example:

- The `[Backend]` section is declared twice → Deckhouse Code will merge it.
- `[Backend][2] @backend-core` specifies **2 mandatory approvals**.
- `[Frontend][3]` requires **3 approvals** from the frontend.
- `[Ruby Optional]` is marked as optional — owners are visible, but their approval is not required.
- the exceptions `!*.lock` and `!**/generated/**` mean that these files do not require approvals at all.

### Exclusions (!)

Used to exclude files within a section:

```plaintext
* @default
!package-lock.json
!**/generated/**
```

After exclusion, the file **cannot be re-included** in the same section.

## Rule processing order

Rules are processed in the following order:

- Deckhouse Code reads the file from top to bottom.
- Within a single section, rules with the same path **override** previous ones.
- If a file fits into several sections, the owners from all these sections are added.
- Sections do not override each other: They are *independent*.

## Who can be an owner (eligible owners)

### 1. Users (via `@username`)

A user is considered a valid owner if:

- They have **direct** membership in the project (Developer, Maintainer, or Owner).
- They have membership in a project group (including inherited: project group, parent groups, ancestors of parent groups).
- They are a direct or inherited member of a group that is directly invited to the project.
- They are a direct but not inherited member of a group that is invited to the project group.
- They are a direct but not inherited member of a group that is invited to an ancestor of the project group.

To sum up, all users who are displayed on the project participants page with the role of developer or higher are eligible.

A user is not considered an owner if:

- they are blocked,
- their access has been revoked,
- their role is lower than Developer.

### 2. Groups (via `@group` or `@group/subgroup`)

A group can be an owner **only if:**

- It is directly invited to the project with the Developer+ role (via “Share group with project”).
- Only *direct members* of this group are considered owners.

Inherited members of parent groups are not considered owners.

### 3. Roles (via `@@role`)

Format:

```console
@@developer
@@maintainer
@@owner
```

Features:

- **Only direct project participants** are taken into account.
- Group members are not included in the calculation.
- Roles are not inherited from each other (`@@developer` does NOT include maintainers).

You can specify multiple roles in one line:

```console
file.md @@developer @@maintainer
```

## Interaction with approval rules

CODEOWNERS and approval rules work independently, but their requirements are cumulative.

Example:

- The `Security` rule requires 1 approver.
- The `[Backend][2]` section requires 2 approvers.
- MR changes the backend file.

Result: **3 approvers** are required.

## Formatting recommendations

When formatting the `CODEOWNERS` file, follow these recommendations:

- First, set broad rules (`* @team`). Then specify more detailed rules (`app/**/*.rb @backend-team`).
- Use sections for better structure and clarity.
- Avoid duplicates — the last rule in a section takes precedence.
- Optional sections are useful for highlighting experts without mandatory approvals.

## Example of the final file

```console
# Global owners
* @default-team

# Exclusions
!yarn.lock
!**/generated/**

[Backend][2]
app/**/*.rb @backend-core

[Frontend][3]
*.vue @frontend-team
*.js @frontend-team

^[Docs]
*.md @docs-team

[Critical Overrides]
/config/production.yml @ops-lead
```
