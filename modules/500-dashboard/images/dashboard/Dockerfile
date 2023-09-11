# Based on https://github.com/kubernetes/dashboard/blob/v2.7.0/aio/Dockerfile
ARG BASE_ALPINE
FROM kubernetesui/dashboard:v2.7.0@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93 as artifact

# to fix dashboard can display cronjobs v1
FROM golang:1.19.12-alpine3.18 as patchbuilder

WORKDIR /src/
RUN apk add --no-cache git patch make zip wget && \
    wget -c https://github.com/kubernetes/dashboard/archive/refs/tags/v2.7.0.zip && \
    unzip v2.7.0.zip
COPY patches/001-fix-cronjob-ver.patch /src/
RUN patch -u dashboard-2.7.0/src/app/backend/resource/cronjob/jobs.go -i 001-fix-cronjob-ver.patch --ignore-whitespace --fuzz 3 && \
    cd dashboard-2.7.0 && go mod download
WORKDIR /src/dashboard-2.7.0/src/app/backend/
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags "-X github.com/kubernetes/dashboard/src/app/backend/client.Version=v2.7.0" -o dashboard .


FROM $BASE_ALPINE

COPY --from=artifact /etc/passwd /etc/passwd
COPY --from=artifact /public /public
COPY --from=artifact /locale_conf.json /locale_conf.json
# COPY --from=artifact /dashboard /dashboard
COPY --from=patchbuilder /src/dashboard-2.7.0/src/app/backend/dashboard /dashboard

# Inject logout button to be able to change user if token authentication is used
ADD ./logout_button.js /public/logout_button.js
ADD ./install_logout.sh /
RUN sh /install_logout.sh && \
    rm /public/logout_button.js && \
    rm /install_logout.sh

USER nonroot
EXPOSE 9090 8443
ENTRYPOINT ["/dashboard"]
