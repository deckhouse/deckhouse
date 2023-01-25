---
title: "The virtualization module: FAQ"
---

## How to store an image in the registry

To store an image in the registry, you need to build a docker image with one `/image` directory in which you should put an image with an arbitrary name.  
The image can be in either `qcow2` or `raw` format.

Example of a `Dockerfile`:

```Dockerfile
FROM scratch
ADD https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img /disk/jammy-server-cloudimg-amd64.img
```
