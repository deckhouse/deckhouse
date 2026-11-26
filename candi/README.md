# base_images.yml
ATTENTION! DO NOT CHANGE FILE `base_images.yml` DIRECTLY!

To update `base_images.yml` in the Deckhouse repo, use the next command (in the repo root):

```bash
make version=DESIRED_VERSION update-base-images-versions
```

# version_map.yml

Kubernetes patch versions in `version_map.yml` are bumped by a script:

```bash
make update-k8s-patch-versions
```

The same target is used by the scheduled `k8s-autoupdate` CI job, which opens a merge request
with the resulting diff.
