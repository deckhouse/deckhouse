---
title: "Working with Git"
permalink: en/code/documentation/user/git.html
---

## Repository cloning

Cloning allows you to copy a remote Git repository to your local machine.  
To clone, run the following command in the terminal:

```bash
git clone <repository-url>
```

Replace `repository-url` with the HTTPS or SSH URL of your repository.

## Getting updates

To fetch the latest changes from the remote repository, navigate to the repository folder on your local machine and run:

```bash
git pull origin <branch-name>
```

Replace `branch-name` with the name of the branch, e.g., `main` or `feature/my-new-feature`.

## Pushing changes

1. Make necessary changes in your codebase.

1. Commit the changes with the command:

   ```bash
   git commit -am "Description of your changes"
   ```

1. Push the changes to the remote repository:

   ```bash
   git push origin <branch-name>
   ```

Replace `branch-name` with the name of the branch to which you are pushing.

## Creating a feature branch

Feature branches are used to develop new features or fixes. Below are the steps to create a feature branch and push it to Deckhouse Code:

1. Clone the repository:

   ```bash
   git clone <repository-url>
   ```

   Replace `repository-url` with the actual repository URL.

1. Change directory to the repository folder:

   ```bash
   cd <path-to-folder-with-repository>
   ```

1. Create a new feature branch:

   ```bash
   git checkout -b <branch-name>
   ```

   Replace `branch-name` with the name of your new branch.
   The `-b` flag tells Git to create the new branch and switch to it immediately.

1. Push the new branch to the remote repository:

   ```bash
   git push origin <branch-name>
   ```

   Make sure the branch appears in the remote repository.  
   To verify, open the project in Deckhouse Code and find the new branch in the branch selection menu.

## Merge request

A merge request is used to combine changes from one branch into another.

Benefits of using merge requests:

1. Code quality control:

   - Allows code review before merging into the main branch.
   - Helps catch errors and maintain consistent coding style.

1. Collaboration organization:

   - Developers can comment on code, suggest fixes, and discuss changes.

1. Testing and CI/CD automation:

   - MRs can integrate with CI/CD pipelines for automatic testing of changes.

1. Change history and documentation:

   - MRs include descriptions of changes, reviewer comments, and discussions.

1. Development flexibility and version control:

   - Enables easy rollback or amendments before merging.

1. Enhanced security:

   - Prevents vulnerabilities through checks and reviews.

### Creating a merge request

1. Navigate to the Merge Requests section:
   - Open your project in Deckhouse Code.
   - In the sidebar, select: "Code" → "Merge requests".

1. Create a new merge request:
   - Click the "New merge request" button.
   - Choose the source branch (with your changes) and the target branch (usually main).
   - Click "Compare branches and continue".

1. Fill in merge request details:
   - Title: briefly describe the changes.
   - Description: provide details and add screenshots if needed.
   - Assignee: assign a user to review the request.

1. Submit the merge request:
   - Click "Create merge request".

### Reviewing and merging the request

The merge can proceed once the following conditions are met:

- The merge request has passed review and received the required approvals.
- There are no conflicts between branches.
- All CI/CD pipelines have successfully completed.

To complete the process, click the "Merge" button on the merge request page.
