# Based on https://github.com/open-policy-agent/gatekeeper/blob/master/Dockerfile
ARG BASE_ALPINE
FROM openpolicyagent/gatekeeper:v3.10.0@sha256:ab55e50d7a188472b87bc336066c217ccde2b123042d0b30b3cb01bb9cfe04aa as base


FROM $BASE_ALPINE

COPY --from=base /manager /bin/gatekeeper

RUN apk add --no-cache ca-certificates

ENTRYPOINT ["/bin/gatekeeper"]


