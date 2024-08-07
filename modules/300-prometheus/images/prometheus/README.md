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

### Federation external labels control

This patch allows to instruct Prometheus to drop external labels from federation source at a scrape time, allowing to export metrics as is.

```yaml
scrape_configs:
- job_name: 'federate'
  honor_labels: true
  metrics_path: '/federate'
  params:
    drop_external_labels:
    - "1"
```

### How-to build prometheus

Current falco version is `PROMETHEUS_VERSION=v2.45.2`.

To build prometheus we need to repush used third-party libs to own registry. To do this we need to run specific Dockerfile:

```dockerfile
ARG BASE_GOLANG_21_BULLSEYE_DEV
FROM $BASE_GOLANG_21_BULLSEYE_DEV
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
docker build --build-arg SOURCE_REPO=https://github.com --build-arg BASE_GOLANG_21_BULLSEYE_DEV=registry.deckhouse.io/base_images/dev-golang:1.21.6-bullseye --build-arg PROMETHEUS_VERSION=${PROMETHEUS_VERSION} -t prometheus-deps .
```

Than copy folders from container:

```shell
docker run --name prometheus-deps -d prometheus-deps bash
docker cp prometheus-deps:/prometheus/prometheus/web/ui/node_modules node_modules
docker cp prometheus-deps:/prometheus/prometheus/web/ui/package-lock.json package-lock.json
docker rm -f prometheus-deps
```

Then commit content to `fox.flant.com/deckhouse/3p/prometheus/prometheus-deps` to the branch `${PROMETHEUS_VERSION}`.
