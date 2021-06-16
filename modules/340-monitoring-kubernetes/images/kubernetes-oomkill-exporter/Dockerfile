# Based on https://github.com/sapcc/kubernetes-oomkill-exporter/blob/0.2.0/Dockerfile
ARG BASE_ALPINE
FROM sapcc/kubernetes-oomkill-exporter:0.2.0@sha256:5e1b57f4ac0b57406ef067da3e83f743d70ff89aa1db717d41af2c699dc12f3a as artifact

FROM $BASE_ALPINE
RUN apk --no-cache add ca-certificates
COPY --from=artifact /kubernetes-oomkill-exporter /

ENTRYPOINT ["/kubernetes-oomkill-exporter"]
CMD ["-logtostderr"]
