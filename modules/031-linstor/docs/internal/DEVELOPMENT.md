---
title: Maintenance of the linstor module 
searchable: false
---

How to update LINSTOR
---------------------

All component versions are hardcoded in Dockerfiles.
The make is done from the official Github repositories.

To get a list of all repositories and current versions, go to the `./images` directory and run:

```
grep -r '^ARG [A-Z_]*_\(GITREPO\|VERSION\)=' | awk '{print $NF}' | sort -u
```

To update component versions, just update the corresponding variables in Dockerfiles.

After the upgrade, don't forget to run the integration tests on the existing devel-cluster:

```
helm -n d8-system test linstor
```
