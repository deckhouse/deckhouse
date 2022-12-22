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
RUN cat /public/logout_button.js >> /public/de/de.main.44ac5dc977e4fc4e.js && \
    cat /public/logout_button.js >> /public/en/en.main.44ac5dc977e4fc4e.js && \
    cat /public/logout_button.js >> /public/es/es.main.44ac5dc977e4fc4e.js && \
    cat /public/logout_button.js >> /public/fr/fr.main.44ac5dc977e4fc4e.js && \
    cat /public/logout_button.js >> /public/ja/ja.main.44ac5dc977e4fc4e.js && \
    cat /public/logout_button.js >> /public/ko/ko.main.44ac5dc977e4fc4e.js && \
    cat /public/logout_button.js >> /public/zh-Hans/zh-Hans.main.44ac5dc977e4fc4e.js && \
    cat /public/logout_button.js >> /public/zh-Hant/zh-Hant.main.44ac5dc977e4fc4e.js && \
    cat /public/logout_button.js >> /public/zh-Hant-HK/zh-Hant-HK.main.44ac5dc977e4fc4e.js && \
    rm /public/logout_button.js

USER nonroot
EXPOSE 9090 8443
ENTRYPOINT ["/dashboard"]
