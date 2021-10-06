ARG BASE_ALPINE
FROM flant/vector:0.16.1-alpine@sha256:c0705a557fcf9022c0b23a50d6eb1d51d33207310c61dd58106aab9b23f77ebb AS vector

FROM $BASE_ALPINE
COPY --from=vector /usr/local/bin/vector /usr/local/bin/vector
RUN mkdir -p /etc/vector \
    && apk update \
    && apk add ca-certificates tzdata \
    && rm -rf /var/cache/apk/*
ENTRYPOINT ["/usr/local/bin/vector"]
