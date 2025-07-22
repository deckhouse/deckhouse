---
title: "Pull Mirroring"
menuTitle: Pull Mirroring
force_searchable: true
description: Configuring pull mirroring in a repository
permalink: en/code/documentation/user/pull-mirroring.html
lang: en
weight: 50
---

## Pull Mirroring  

Allows you to configure repository mirroring.  
On the project page, go to **Settings** → **Repository** → **Mirroring repositories**.

> 📘 If the repository is empty, you need to import it first, since all hooks are triggered during mirroring, and pulling a large repository may put a load on system components.

---

### Instructions for Configuring Repository Pull Mirroring

#### 1. Go to the Project Page  

1. Open your project in GitLab.  
2. In the left menu, click **Settings** → **Repository**.  
3. Go to the **Mirroring repositories** section.

---

#### 2. Repository URL  

- Credentials specified in the URL will be ignored — you need to provide them below in the authentication section.

---

#### 3. Configure Authentication  

1. In the **Authentication method** section, select **Username and password** if the repository requires HTTP(S) authentication.  
2. Provide:  
   - **Username**  
   - **Password**  
3. If you use SSH mirroring, specify the username (usually `git`). After the mirror is created, an SSH key will be generated and used to access the repository.

---

#### 4. LFS (Large File Storage) Considerations  

- In **pull mirroring**, LFS objects will be created **only** if LFS is enabled in your GitLab project.

---

### How It Works  

- Mirroring jobs are scheduled by the `Projects::PullMirrorScheduleWorker` job once per hour.  
- Each project's mirroring is triggered every 6 hours.  
- The maximum number of retry attempts upon failure is 5. Clicking "Update now" resets the failure counter.  
- If the Sidekiq job crashes, its status will be updated after 3 hours, and the job will be re-queued.
