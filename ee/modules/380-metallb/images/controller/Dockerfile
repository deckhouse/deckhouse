# based on https://raw.githubusercontent.com/metallb/metallb/v0.13.5/config/manifests/metallb-native.yaml
ARG BASE_ALPINE
FROM $BASE_ALPINE
COPY --from=quay.io/metallb/controller:v0.13.7@sha256:0e486995b27d62433e728f7b266fb8b7b7f4fd65e1b751bef17fe56e76653bf1 /controller /
ENTRYPOINT ["/controller"]
