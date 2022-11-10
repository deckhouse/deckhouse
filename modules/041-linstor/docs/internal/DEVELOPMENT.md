---
title: Maintenance of the linstor module 
searchable: false
---

How to update LINSTOR
---------------------

All component versions are hardcoded in Dockerfiles.
The make is done from the official Github repositories.

To get a list of all repositories and current versions, go to the `./images` directory and run:

```shell
grep -r '^ARG [A-Z_]*_\(GITREPO\|VERSION\|COMMIT_REF\)=' | awk '{print $NF}' | sort -u
```

To update component versions, just update the corresponding variables in Dockerfiles.  
This can also be done by running helper script.

```shell
hack/update.sh
```

After the upgrade, don't forget to run the integration tests on the existing devel-cluster:

```shell
helm -n d8-system test linstor
```
