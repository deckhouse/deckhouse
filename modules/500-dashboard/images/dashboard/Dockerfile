# Based on https://github.com/kubernetes/dashboard/blob/v2.2.0/aio/Dockerfile
ARG BASE_ALPINE
FROM kubernetesui/dashboard:v2.2.0@sha256:148991563e374c83b75e8c51bca75f512d4f006ddc791e96a91f1c7420b60bd9 as artifact

FROM $BASE_ALPINE

COPY --from=artifact /etc/passwd /etc/passwd
COPY --from=artifact /public /
COPY --from=artifact /locale_conf.json /
COPY --from=artifact /dashboard /

USER nonroot
EXPOSE 9090 8443
ENTRYPOINT ["/dashboard", "--insecure-bind-address=0.0.0.0", "--bind-address=0.0.0.0"]
