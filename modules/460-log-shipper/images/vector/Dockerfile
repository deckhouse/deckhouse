ARG BASE_ALPINE
FROM flant/vector:0.14.0-alpine AS vector

FROM $BASE_ALPINE
COPY --from=vector /usr/local/bin/vector /usr/local/bin/vector
RUN mkdir -p /etc/vector \
    && apk update \
    && apk add ca-certificates tzdata \
    && rm -rf /var/cache/apk/*
ENTRYPOINT ["/usr/local/bin/vector"]
