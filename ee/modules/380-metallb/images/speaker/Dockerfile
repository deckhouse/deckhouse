# based on https://raw.githubusercontent.com/metallb/metallb/v0.13.5/config/manifests/metallb-native.yaml
FROM $BASE_ALPINE
COPY --from=quay.io/metallb/speaker:v0.13.5@sha256:aa593712b8372dcde97954b1be422f66a7cad39f782a80ea5fee9edf125ed4e3 /speaker /
ENTRYPOINT ["/speaker"]
