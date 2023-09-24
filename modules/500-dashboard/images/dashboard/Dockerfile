# Based on https://github.com/kubernetes/dashboard/blob/v2.7.0/aio/Dockerfile
ARG BASE_ALPINE
ARG BASE_GOLANG_19_ALPINE
FROM kubernetesui/dashboard:v2.7.0@sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93 as artifact

# to fix dashboard can display cronjobs v1
FROM $BASE_GOLANG_19_ALPINE as patchbuilder

ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /src/
RUN apk add --no-cache git patch make zip && \
    git clone ${SOURCE_REPO}/kubernetes/dashboard && \
    cd dashboard && \
    git config --global user.email "builder@deckhouse.io" && \
    git checkout v2.7.0 && \
    # Use batch/v1 for fetching cronjob jobs https://github.com/kubernetes/dashboard/pull/7465
    git cherry-pick f79f3a5e25df152df6164ddfaf2ffc1f09e5058f && \
    go mod download
WORKDIR /src/dashboard/src/app/backend/
RUN go build -a -installsuffix cgo -ldflags "-X github.com/kubernetes/dashboard/src/app/backend/client.Version=v2.7.0" -o dashboard .


FROM $BASE_ALPINE

COPY --from=artifact /etc/passwd /etc/passwd
COPY --from=artifact /public /public
COPY --from=artifact /locale_conf.json /locale_conf.json
COPY --from=patchbuilder /src/dashboard/src/app/backend/dashboard /dashboard

# Inject logout button to be able to change user if token authentication is used
ADD ./logout_button.js /public/logout_button.js
ADD ./install_logout.sh /
RUN sh /install_logout.sh && \
    rm /public/logout_button.js && \
    rm /install_logout.sh

USER nonroot
EXPOSE 9090 8443
ENTRYPOINT ["/dashboard"]
