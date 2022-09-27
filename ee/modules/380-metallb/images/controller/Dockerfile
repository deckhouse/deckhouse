# based on https://raw.githubusercontent.com/metallb/metallb/v0.13.5/config/manifests/metallb-native.yaml
ARG BASE_ALPINE
FROM quay.io/metallb/controller:v0.13.5@sha256:44e7d8b0f20d944156bb6f6a9df0230305a2e420c6f5fd2f3ecb3f6efee9b4ff as artifact

FROM $BASE_ALPINE
COPY --from=artifact /controller /
ENTRYPOINT ["/controller"]
