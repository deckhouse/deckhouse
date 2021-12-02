# Based on https://github.com/kubernetes/dashboard/blob/v2.2.0/aio/Dockerfile
ARG BASE_ALPINE
FROM kubernetesui/dashboard:v2.2.0@sha256:148991563e374c83b75e8c51bca75f512d4f006ddc791e96a91f1c7420b60bd9 as artifact

FROM $BASE_ALPINE

COPY --from=artifact /etc/passwd /etc/passwd
COPY --from=artifact /public /public
COPY --from=artifact /locale_conf.json /locale_conf.json
COPY --from=artifact /dashboard /dashboard

# Inject logout button to be able to change user if token authentication is used
ADD ./logout_button.js /public/logout_button.js
RUN cat /public/logout_button.js >> /public/en/en.main.09bf52db2dbc808e7279.js

USER nonroot
EXPOSE 9090 8443
ENTRYPOINT ["/dashboard"]
