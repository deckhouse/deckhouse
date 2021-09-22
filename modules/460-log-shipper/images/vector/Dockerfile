ARG BASE_ALPINE
FROM flant/vector:0.15.4-alpine@sha256:78cd243e9a8ebc279a8cc8586398fe0b12f089db60ac62c90d39910e4057ea7f AS vector

FROM $BASE_ALPINE
COPY --from=vector /usr/local/bin/vector /usr/local/bin/vector
RUN mkdir -p /etc/vector \
    && apk update \
    && apk add ca-certificates tzdata \
    && rm -rf /var/cache/apk/*
ENTRYPOINT ["/usr/local/bin/vector"]
