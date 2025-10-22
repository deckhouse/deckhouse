---
title: "Pull Mirroring"
menuTitle: Pull Mirroring
force_searchable: true
description: Configuring pull mirroring in a repository
permalink: en/code/documentation/user/pull-mirroring.html
weight: 50
---

Allows you to configure repository mirroring. On the project page, go to "Settings" → "Repository" → "Mirroring repositories".

If the repository is empty, import it first. All hooks are triggered during mirroring, and pulling a large repository may significantly impact system performance.

## Configuring pull repository mirroring

1. Go to the project page:

   - Open your project in the Deckhouse Code interface.  
   - In the left-hand menu, select "Settings" → "Repository".  
   - Scroll down to the "Mirroring repositories" section.

1. Specify the repository URL:
   - Credentials in the URL are ignored — use the fields in the "Authentication" block below for authorization.

1. Configure authentication:
   - In the "Authentication method" field, select "Username and password" if using HTTP(S) access.  
   - Provide the following:  
      - "Username" — your username;  
      - "Password" — your password or access token.  
   - If using SSH mirroring, specify the username (typically `git`). After saving the configuration, Deckhouse Code will generate an SSH key to be used for access.

### LFS (Large File Storage) specifics

When using pull mirroring, LFS objects will be fetched **only** if LFS is enabled in the target Deckhouse Code project:

- Mirroring jobs are scheduled once per hour ("Projects::PullMirrorScheduleWorker").  
- Each mirror is updated no more than once every 6 hours.  
- The maximum number of retry attempts is 5. Clicking the "Update now" button resets the failure counter.  
- If a Sidekiq job fails unexpectedly, its status will update after 3 hours and a new synchronization attempt will be queued.
