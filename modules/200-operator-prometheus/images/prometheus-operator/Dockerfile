FROM golang:1.12
RUN apt update && apt install -qfy \
  bash make git patch ca-certificates openssh-client openssl
RUN mkdir /coreos && cd /coreos \
  && git clone -b "v0.38.0" --single-branch https://github.com/coreos/prometheus-operator
WORKDIR /coreos/prometheus-operator
COPY tls-config-and-bearer-token.patch scrape_params.patch ./
RUN patch -p1 < tls-config-and-bearer-token.patch && \
    patch -p1 < scrape_params.patch && \
    make operator

# todo: подобрать подходящий образ
FROM quay.io/prometheus/busybox:latest
COPY --from=0 /coreos/prometheus-operator/operator /bin/operator
# On busybox 'nobody' has uid `65534'
USER 65534
ENTRYPOINT ["/bin/operator"]
