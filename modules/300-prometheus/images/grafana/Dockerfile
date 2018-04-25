FROM grafana/build-container:v0.1
RUN mkdir -p /go/src/github.com/grafana/grafana && \
    wget https://github.com/grafana/grafana/archive/v5.1.0-beta1.tar.gz -O - | tar -xz --strip-components=1 -C /go/src/github.com/grafana/grafana
WORKDIR /go/src/github.com/grafana/grafana
RUN make deps
COPY ./extra_vars.patch .
RUN patch -p1 < ./extra_vars.patch
RUN bash -l -c "go run build.go build package" && \
    mv dist/*.deb /grafana.deb

FROM debian:jessie
COPY --from=0 /grafana.deb /
RUN apt-get update && \
    apt-get -y --no-install-recommends install libfontconfig curl ca-certificates && \
    apt-get clean && \
    dpkg -i /grafana.deb && \
    rm /grafana.deb && \
    curl -L https://github.com/tianon/gosu/releases/download/1.10/gosu-amd64 > /usr/sbin/gosu && \
    chmod +x /usr/sbin/gosu && \
    apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/*
VOLUME ["/var/lib/grafana", "/var/log/grafana", "/etc/grafana"]
EXPOSE 3000
COPY ./run.sh /run.sh
WORKDIR /
ENTRYPOINT ["/run.sh"]
