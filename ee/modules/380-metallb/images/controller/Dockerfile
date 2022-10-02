# based on https://raw.githubusercontent.com/metallb/metallb/v0.13.5/config/manifests/metallb-native.yaml
FROM $BASE_ALPINE
COPY --from=quay.io/metallb/controller:v0.13.5@sha256:44e7d8b0f20d944156bb6f6a9df0230305a2e420c6f5fd2f3ecb3f6efee9b4ff /controller /
ENTRYPOINT ["/controller"]
