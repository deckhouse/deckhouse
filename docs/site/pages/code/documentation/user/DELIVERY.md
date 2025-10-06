---
title: "Delivery (CI/CD)"
permalink: en/code/documentation/user/delivery.html
---

Deckhouse Code uses the `.gitlab-ci.yml` file to automate testing, build, and deployment processes through CI/CD pipelines. This enables the following functionality:

- Automatic code checks on every commit.
- Flexible configuration of execution stages and jobs.
- Parallel job execution to speed up build and test processes.
- Custom test steps tailored to project requirements.
- Pipeline triggers on various repository events such as commits, merge requests, tag creation, and more.
