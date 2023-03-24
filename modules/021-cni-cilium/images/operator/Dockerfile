# Based on https://github.com/cilium/cilium/blob/v1.12.7/images/operator/Dockerfile
ARG BASE_ALPINE
# https://github.com/cilium/cilium/releases/tag/v1.12.7
FROM quay.io/cilium/operator:v1.12.7@sha256:feb89658583c5e35197aebae147eb9888db96fe56e5db18897b788d179452e35 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/bin/cilium-operator /usr/bin/cilium-operator

RUN apk add --no-cache ca-certificates

USER nobody
ENTRYPOINT ["/usr/bin/cilium-operator"]
