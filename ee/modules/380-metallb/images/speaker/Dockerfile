# based on https://raw.githubusercontent.com/metallb/metallb/v0.13.5/config/manifests/metallb-native.yaml
ARG BASE_ALPINE
FROM $BASE_ALPINE
COPY --from=quay.io/metallb/speaker:v0.13.7@sha256:aeab5aeeb25c7ca31a1c468a35b1db4513cd6199bf376bdf19ff5689d2ebad28 /speaker /
ENTRYPOINT ["/speaker"]
