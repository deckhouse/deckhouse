### How-to build prometheus

Current prometheus version is `PROMETHEUS_VERSION=v2.55.1`.

To build prometheus we need to repush used third-party libs to own registry. To do this we need to run specific Dockerfile:

```dockerfile
ARG BASE_GOLANG_23_BULLSEYE
FROM $BASE_GOLANG_23_BULLSEYE
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
export PROMETHEUS_VERSION=v2.55.1
# Check the image:tag used in the runner that will execute `npm run build`,
# and for consistency, ensure they are the same.”
export BASE_GOLANG_23_BULLSEYE=golang:1.23.6-bullseye
docker build --build-arg SOURCE_REPO=https://github.com --build-arg BASE_GOLANG_23_BULLSEYE=${BASE_GOLANG_23_BULLSEYE} --build-arg PROMETHEUS_VERSION=${PROMETHEUS_VERSION} -t prometheus-deps . --progress=plain --no-cache
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
