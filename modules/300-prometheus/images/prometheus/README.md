## Patches

### Sample limit annotation

Limit the number of metrics which Prometheus scrapes from a target.  

```yaml
metadata:
  annotations:
    prometheus.deckhouse.io/sample-limit: "5000"
```

### Successfully sent metric

Exports gauge metric with the count of successfully sent alerts. 

### How-to build prometheus

Current prometheus version is `PROMETHEUS_VERSION=v2.55.1`.

To build prometheus we need to repush used third-party libs to own registry. To do this we need to run specific Dockerfile:

```dockerfile
ARG BASE_GOLANG_22_BULLSEYE_DEV
FROM $BASE_GOLANG_22_BULLSEYE_DEV
ARG PROMETHEUS_VERSION
ARG SOURCE_REPO

ENV SOURCE_REPO=${SOURCE_REPO} \
    PROMETHEUS_VERSION=${PROMETHEUS_VERSION}
    
RUN mkdir /prometheus && cd /prometheus &&\
    git clone -b "${PROMETHEUS_VERSION}" --single-branch {{ $.SOURCE_REPO }}/prometheus/prometheus &&\
    cd /prometheus/prometheus/web/ui &&\
    npm install
```

To run Dockerfile exec the command:

```shell
export PROMETHEUS_VERSION=v2.53.2
# Check the image:tag used in the runner that will execute `npm run build`,
# and for consistency, ensure they are the same.”
export BASE_GOLANG_22_BULLSEYE_DEV=registry.deckhouse.io/base_images/dev-golang:1.22.8-bullseye@sha256:b79c06949dd2a4e19b900b1c29372219cfb0418109439c8b38fc485d26bbccdb
docker build --build-arg SOURCE_REPO=https://github.com --build-arg BASE_GOLANG_22_BULLSEYE_DEV=${BASE_GOLANG_22_BULLSEYE_DEV} --build-arg PROMETHEUS_VERSION=${PROMETHEUS_VERSION} -t prometheus-deps . --progress=plain --no-cache

```

Than copy folders from container:

```shell
docker run --name prometheus-deps -d prometheus-deps 
docker cp prometheus-deps:/prometheus/prometheus/web/ . 
docker rm -f prometheus-deps
```

Before commiting your changes, remove the `.gitignore` files, so that all
changes made by `npm install` are included into the repostiory.

```shell
find . -name ".gitignore" -type f -delete
```

Then commit content to `fox.flant.com/deckhouse/3p/prometheus/prometheus-deps` to the branch `${PROMETHEUS_VERSION}`.
