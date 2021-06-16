ARG BASE_ALPINE
ARG BASE_GOLANG_BUSTER
FROM $BASE_GOLANG_BUSTER as artifact
RUN apt update && apt install -qfy \
  bash make git patch ca-certificates openssh-client openssl
RUN mkdir /coreos && cd /coreos \
  && git clone -b "v0.38.0" --single-branch https://github.com/coreos/prometheus-operator
WORKDIR /coreos/prometheus-operator
COPY prometheus-probes.patch tls-config-and-bearer-token.patch scrape_params.patch ./
RUN patch -p1 < prometheus-probes.patch && \
    patch -p1 < tls-config-and-bearer-token.patch && \
    patch -p1 < scrape_params.patch && \
    make operator

FROM $BASE_ALPINE
COPY --from=artifact /coreos/prometheus-operator/operator /bin/operator
USER 65534
ENTRYPOINT ["/bin/operator"]
