ARG BASE_ALPINE
ARG BASE_GOLANG_17_BUSTER
FROM $BASE_GOLANG_17_BUSTER as artifact

RUN apt update && apt install -qfy \
  bash make git patch ca-certificates openssh-client openssl
RUN mkdir /prometheus-operator && cd /prometheus-operator \
  && git clone -b "v0.56.3" --single-branch https://github.com/prometheus-operator/prometheus-operator.git

WORKDIR /prometheus-operator/prometheus-operator
COPY patches/scrape-params.patch ./
COPY patches/scrape-timestamp-align.patch ./
RUN patch -p1 < scrape-timestamp-align.patch && \
    patch -p1 < scrape-params.patch && \
    go mod tidy && \
    make operator

FROM $BASE_ALPINE
COPY --from=artifact /prometheus-operator/prometheus-operator/operator /bin/operator
USER 65534
ENTRYPOINT ["/bin/operator"]
