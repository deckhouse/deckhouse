# Cloud Layout Tests

### If something went wrong, and a CI job became canceled/failed.

Example for Yandex Cloud, but it is also suitable for other clouds:
```bash
docker run \
  -ti \
  --entrypoint /bin/bash \
  -v /deckhouse:/deckhouse \
  -v "$PWD/testing/cloud_layouts:/deckhouse/testing/cloud_layouts" \
  -v "$PWD/layouts-tests-tmp:/tmp" \
  -w /deckhouse dev-registry.deckhouse.io/sys/deckhouse-oss/install:master
```
```bash
dhctl destroy \
  --ssh-agent-private-keys testing/cloud_layouts/Yandex.Cloud/WithoutNAT/sshkey \
  --ssh-user ubuntu \
  --ssh-host 178.154.227.70 \
  --yes-i-am-sane-and-i-understand-what-i-am-doing
```
