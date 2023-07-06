# Based on https://github.com/kubernetes/dashboard/blob/v2.7.0/aio/Dockerfile
ARG BASE_ALPINE
FROM kubernetesui/dashboard:v2.7.0@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93 as artifact

FROM $BASE_ALPINE

COPY --from=artifact /etc/passwd /etc/passwd
COPY --from=artifact /public /public
COPY --from=artifact /locale_conf.json /locale_conf.json
COPY --from=artifact /dashboard /dashboard

# Inject logout button to be able to change user if token authentication is used
ADD ./logout_button.js /public/logout_button.js
ADD ./install_logout.sh /
RUN sh /install_logout.sh && \
    rm /public/logout_button.js && \
    rm /install_logout.sh

USER nonroot
EXPOSE 9090 8443
ENTRYPOINT ["/dashboard"]
