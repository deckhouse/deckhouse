# Based on https://github.com/docker-library/redis/blob/master/5/alpine/Dockerfile
ARG BASE_ALPINE

FROM redis:5.0.10-alpine3.12@sha256:cab1de051c243a956749397af796d50bf235c7183e88bc4667259f66cad4f748 as artifact

FROM $BASE_ALPINE
RUN addgroup -S -g 1000 redis && adduser -S -G redis -u 999 redis
RUN apk add --no-cache 'su-exec>=0.2' tzdata
RUN mkdir /data && chown redis:redis /data

VOLUME /data
WORKDIR /data

COPY --from=artifact /usr/local/bin/ /usr/local/bin/
ENTRYPOINT ["docker-entrypoint.sh"]

EXPOSE 6379
CMD ["redis-server"]
