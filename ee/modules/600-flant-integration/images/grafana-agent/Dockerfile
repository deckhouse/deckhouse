# Based on https://github.com/grafana/agent/blob/v0.13.0/cmd/agent/Dockerfile
ARG BASE_DEBIAN
FROM grafana/agent:v0.18.2@sha256:26018914a1853017acfe5332c8c2639abf7118cead1f383636bd91900ea50b97 as artifact

FROM $BASE_DEBIAN
# Backports repo required to get a libsystemd version 246 or newer which is required to handle journal +ZSTD compression
RUN echo "deb http://deb.debian.org/debian buster-backports main" >> /etc/apt/sources.list
RUN apt-get update && apt-get install -t buster-backports -qy libsystemd-dev && \
  apt-get install -qy tzdata ca-certificates && \
  rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY --from=artifact /bin/agent /bin/
COPY --from=artifact /etc/agent/agent.yaml /etc/agent/

ENTRYPOINT ["/bin/agent"]
CMD ["--config.file=/etc/agent/agent.yaml", "--prometheus.wal-directory=/etc/agent/data"]
