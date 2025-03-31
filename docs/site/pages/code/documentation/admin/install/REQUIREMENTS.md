---
title: "Requirements"
permalink: en/code/documentation/admin/install/requirements.html
---

## Terms and definitions

| **Term**                      | **Definition**                                                                   |
|--------------------------------|----------------------------------------------------------------------------------|
| **Module**                     | A means of logically dividing software into blocks, each performing a specific task. |
| **ModuleConfig**               | A special file for the Kubernetes orchestrator containing the configuration description of a specific module. |
| **Software (SW)**              | Software.                                                                         |
| **MRA (Merge Request Approval)**| A mechanism for fine-tuning the required reviewers for merge request approval.     |

## Prerequisites for installation

Before installing Deckhouse Code, complete the following steps:

1. Install Deckhouse Kubernetes Platform. The detailed installation process of the platform is provided in the link. Without this platform, Deckhouse Code cannot operate, as it provides the necessary orchestration and module management.

1. Enable the Deckhouse Code component. After installing the Deckhouse Kubernetes Platform, enable the Deckhouse Code component. The component is connected as a module using the `ModuleConfig`. Ensure that the configuration parameters meet the infrastructure requirements.

## System requirements

For the correct operation of Deckhouse Code, the following are required:

1. PostgreSQL with the following parameters:

   -*Version: 15.0 or higher;  
   - Extensions:  `btree_gist`,  `pg_trgm`,  `plpgsql` (created automatically if the user has `SUPERUSER` rights);  
   - Disk space: at least 50 GB (more is recommended for intensive workloads).

1. Redis with the following parameters:

   - Version: 7.0 or higher;  
   - Recommended architecture: Redis + Sentinel for high availability;  
   - ACL (Access Control List) settings: the user must have the following permissions: `-@dangerous +role`.

1. Configured compatible S3 storage for storing artifacts, CI/CD files, and other data.

   - Supported providers:
     - YCloud;  
     - AWS;  
     - AzureRM;  
     - Generic S3 (S3-compatible service).  

   - Required policies (permissions) for the user:
     - `s3:ListAllMyBuckets`;
     - `s3:ListBucket`;
     - `s3:GetObject`;
     - `s3:PutObject`;
     - `s3:DeleteObject`.

   - Default buckets:
     - `d8-code-artifacts`;
     - `d8-code-ci-secure-files`;
     - `d8-code-mr-diffs`;
     - `d8-code-git-lfs`;
     - `d8-code-packages`;
     - `d8-code-terraform-state`;
     - `d8-code-uploads`;
     - `d8-code-pages` (if the `Pages` functionality is enabled).

     > If the user does not have permissions to create buckets, they must be created manually, taking into account the specified `bucketPrefix`.
