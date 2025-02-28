## How to build static assets for hubble-ui-frontend

```
# Based on https://github.com/cilium/cilium/blob/v1.14.14/install/kubernetes/cilium/values.yaml#L1375
# and https://github.com/cilium/hubble-ui/blob/v0.13.1/Dockerfile
```

Move to the folder containing the "deckhouse" repository.
```shell
cd ~/git/deckhouse/deckhouse
```

Create `Dockerfile.static` at `modules/500-cilium-hubble/images/ui/` with the following content:

```shell
cat > modules/500-cilium-hubble/images/ui/Dockerfile.static <<"EOF"
ARG NODE_BASE_IMAGE
FROM --platform=linux/amd64 $NODE_BASE_IMAGE

ARG SOURCE_REPO
ARG HUBBLE_UI_VERSION
ENV SOURCE_REPO=${SOURCE_REPO} HUBBLE_UI_VERSION=${HUBBLE_UI_VERSION}
ENV TARGETOS=linux TARGETARCH=amd64

RUN apk add --no-cache git bash
RUN mkdir -p /src && cd /src
RUN --mount=type=ssh git clone --depth 1 --branch ${HUBBLE_UI_VERSION} ${SOURCE_REPO}/cilium/hubble-ui.git /src
COPY patches/ /patches/
WORKDIR /src
RUN git apply /patches/*.frontend.patch --verbose
RUN npm --target_arch=${TARGETARCH} install
ENV NODE_ENV=production
RUN npm run build
RUN chown -R 64535:64535 /src/server/public
EOF
```

To build this Docker image, use the following commands:

```shell
docker build \
--build-arg NODE_BASE_IMAGE=registry.deckhouse.io/base_images/node:20.11.0-alpine3.18 \
--build-arg SOURCE_REPO=https://github.com \
--build-arg HUBBLE_UI_VERSION="v0.13.1" \
-t hubble-ui-static-artifact \
-f modules/500-cilium-hubble/images/ui/Dockerfile.static \
modules/500-cilium-hubble/images/ui
```

Than copy folder `/src/server/public` from container to folder `500-cilium-hubble/images/ui/static`:

```shell
docker run --name=hubble-ui-static-assets -d hubble-ui-static-artifact sh
rm -rf modules/500-cilium-hubble/images/ui/static/*
docker cp hubble-ui-static-assets:/src/server/public/. modules/500-cilium-hubble/images/ui/static
```

Please don't forget to delete the temporary Dockerfile that we created.

```shell
docker rm -f hubble-ui-static-assets
rm -rf modules/500-cilium-hubble/images/ui/Dockerfile.static
```
