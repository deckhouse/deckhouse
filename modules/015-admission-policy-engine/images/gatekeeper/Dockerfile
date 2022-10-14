# Based on https://github.com/open-policy-agent/gatekeeper/blob/master/Dockerfile
ARG BASE_ALPINE
FROM openpolicyagent/gatekeeper:v3.9.0@sha256:fe1ef3bb611456d6bf301f9f2a219306564ee9ad17269f78da59eb0afb6c956a as base


FROM $BASE_ALPINE

COPY --from=base /manager /bin/gatekeeper

RUN apk add --no-cache ca-certificates

ENTRYPOINT ["/bin/gatekeeper"]


