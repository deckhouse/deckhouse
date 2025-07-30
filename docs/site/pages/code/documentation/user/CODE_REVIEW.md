---
title: "Working with the codebase and Code Review"
permalink: en/code/documentation/user/code-review.html
---

## Merge requests (MR)

A key tool for ensuring code quality before merging changes into the main branch.

Main features:

- Creating an MR after committing to a feature branch.
- Automatic comparison of changes with the target branch.
- Built-in discussions and code review.
- Support for auto-merge after successful checks (CI/CD, review).
- Visual conflict display and tools for resolution.

## Diff view

Allows visual comparison between code versions.

Supported modes:

- Standard diff — commit-by-commit comparison.
- Side-by-side diff — line-by-line comparison.
- Inline diff — highlights changes within lines.

## Comments and discussions

Tools for collaborative work on code changes.

Features:

- Commenting on specific lines of code.
- Group discussions with user mentions.
- "Mark as resolved" functionality for closing threads.
- Automatic creation based on comments.

## Event notifications

Notification system for project events.

Supported notifications:

- Email alerts about new commits, MRs, and comments.
- Integration with external messengers: Slack, Mattermost, Telegram (configurable alerts).

## Merge request approvals

A quality control mechanism that enforces required approvals before merging.

Features:

- Configurable minimum number of approvals.
- Assigning required reviewers.
- Support for auto-approval based on conditions.
- Merge blocking if required approvals are missing.
- Integration with the Codeowners mechanism for mandatory review by code owners.
- Automatic reset of approvals on MR changes.
- Logging of all approval-related actions.

## Codeowners

A mechanism for assigning responsibility over parts of the codebase.

Functionality:

- Defining owners for specific files and directories.
- Using a `CODEOWNERS` file to declare ownership.
- Automatically assigning owners as MR reviewers.
- Integration with the approvals system: owners become mandatory reviewers.
- Support for wildcard path patterns (`*`, `**`) and owner groups.
- Ability to restrict changes in protected branches to owners only.
