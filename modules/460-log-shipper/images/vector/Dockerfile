ARG BASE_ALPINE
FROM flant/vector:0.14.1-alpine@sha256:563f5dde722f70724837d51341afbaefb953bf3c1d725cb28ef660fe7bb5c1d5 AS vector

FROM $BASE_ALPINE
COPY --from=vector /usr/local/bin/vector /usr/local/bin/vector
RUN mkdir -p /etc/vector \
    && apk update \
    && apk add ca-certificates tzdata \
    && rm -rf /var/cache/apk/*
ENTRYPOINT ["/usr/local/bin/vector"]
