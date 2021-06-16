# Based on https://github.com/prometheus/pushgateway/blob/v0.9.0/Dockerfile
ARG BASE_ALPINE
FROM prom/pushgateway:v0.9.0@sha256:28e98ee43a045c180dda7a35c929cb0016dd71bf829d42a35cb6acce586636f2 as artifact

FROM $BASE_ALPINE
COPY --from=artifact --chown=nobody:nogroup /bin/pushgateway /bin/

EXPOSE 9091
RUN mkdir -p /pushgateway && chown nobody:nogroup /pushgateway
WORKDIR /pushgateway

USER 65534

ENTRYPOINT [ "/bin/pushgateway" ]
