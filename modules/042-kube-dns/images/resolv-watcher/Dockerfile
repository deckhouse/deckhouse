ARG BASE_ALPINE
FROM $BASE_ALPINE
RUN apk add --no-cache bash sed curl inotify-tools
COPY resolv-conf-check /
ENTRYPOINT /resolv-conf-check
