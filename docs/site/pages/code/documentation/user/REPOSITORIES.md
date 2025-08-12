---
title: "Managing Git repositories"
permalink: en/code/documentation/user/repositories.html
---

## Code storage and versioning

Deckhouse Code provides tools for working with Git repositories. Each project includes a repository for storing source code and tracking change history:

- Full support for the distributed Git version control system.
- Execution of standard commands: clone (`git clone`), commit (`git commit`), push (`git push`), pull (`git pull`).
- Automatic tracking and preservation of change history.

## Fork support

A fork is an independent copy of a repository, intended for development without affecting the main codebase:

1. A developer creates a fork of the main repository.
1. Makes changes in their fork.
1. Submits a merge request (MR) to the original repository.
1. After review, the changes can be merged.

This approach is widely used in open source projects and when access to the main repository is restricted.

## Working with branches and merging

Branches allow you to develop new features and fixes in isolation from the main codebase:

- Create branches: `git branch <name>`, `git checkout -b <name>`.
- Switch between branches: `git checkout <branch>`.
- Merge changes: `git merge <branch>`.
- Support for protected branches — changes must go through a merge request.
- Automatic deletion of merged branches (if enabled).

## Built-in web editor

Allows you to modify code directly in the browser without needing to clone the repository:

- Create and edit files through the web interface.
- Automatically create commits when saving changes.
- Markdown support with preview capability.

## Git LFS (Large File Storage) support

Git LFS enables efficient management of large files, such as:

- Images;
- Video;
- Audio files;
- Machine learning datasets.

Git LFS replaces the contents of large files with references, while the actual files are stored outside the Git repository. This reduces load on commit history and improves repository performance.
