ARG BASE_ALPINE
FROM stakater/reloader:v0.0.124@sha256:a793387fb7969f2648a5d5c97b656d4941f85ab81ff3ec2e9a928d511928fa97 AS reloader

FROM $BASE_ALPINE
COPY --from=reloader /manager /usr/local/bin/manager
ENTRYPOINT ["/usr/local/bin/manager"]
