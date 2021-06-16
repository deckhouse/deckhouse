# Based on https://github.com/trickstercache/trickster/blob/v1.1.5/deploy/Dockerfile
ARG BASE_ALPINE
FROM tricksterio/trickster:1.1.5@sha256:c6cbeee1651c78d479a08f3779e036e5d8e469d909bb78901c83e02121d4bfdf as artifact

FROM $BASE_ALPINE
COPY --from=artifact /usr/local/bin/trickster /usr/local/bin/
COPY --from=artifact /etc/trickster/trickster.conf /etc/trickster/
RUN chown nobody /usr/local/bin/trickster
RUN chmod +x /usr/local/bin/trickster

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

USER nobody
ENTRYPOINT ["trickster"]
