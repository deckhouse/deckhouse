ARG BASE_ALPINE
FROM flant/vector:0.15.3-alpine@sha256:86e25094d5d0285c45ace4738a8e9ccff491f0735b1e3bd8f8aa6b998af5abb9 AS vector

FROM $BASE_ALPINE
COPY --from=vector /usr/local/bin/vector /usr/local/bin/vector
RUN mkdir -p /etc/vector \
    && apk update \
    && apk add ca-certificates tzdata \
    && rm -rf /var/cache/apk/*
ENTRYPOINT ["/usr/local/bin/vector"]
