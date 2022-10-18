ARG BASE_ALPINE
FROM stakater/reloader:v0.0.118@sha256:2d423cab8d0e83d1428ebc70c5c5cafc44bd92a597bff94007f93cddaa607b02 AS reloader

FROM $BASE_ALPINE
COPY --from=reloader /manager /usr/local/bin/manager
ENTRYPOINT ["/usr/local/bin/manager"]
