ARG BASE_ALPINE
ARG BASE_DEBIAN
FROM grafana/build-container:1.2.19
RUN mkdir -p /go/src/github.com/grafana/grafana && \
    curl -LSs https://github.com/grafana/grafana/archive/v7.0.6.tar.gz | tar -xz --strip-components=1 -C /go/src/github.com/grafana/grafana
WORKDIR /go/src/github.com/grafana/grafana
RUN make deps
COPY ./extra_vars.patch .
COPY ./folders.patch .
COPY ./fix_thin_bars.patch .
RUN patch -p1 < ./extra_vars.patch && patch -p1 < ./folders.patch
RUN patch -p1 < ./fix_thin_bars.patch
RUN bash -c "go run build.go build package" && \
    mv dist/*.deb /grafana.deb

FROM $BASE_ALPINE
WORKDIR /plugin
RUN apk add --no-cache git && \
    cd /plugin && \
    git clone --branch v0.3.0 https://github.com/flant/grafana-statusmap.git .

FROM $BASE_DEBIAN
COPY --from=0 /grafana.deb /
COPY --from=1 /plugin/dist /plugin-dist
RUN apt-get update && \
    apt-get -y --no-install-recommends install libfontconfig curl ca-certificates && \
    apt-get clean && \
    dpkg -i /grafana.deb && \
    rm /grafana.deb && \
    apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/* && \
    GF_INSTALL_PLUGINS="grafana-image-renderer,petrslavotinek-carpetplot-panel,vonage-status-panel,btplc-status-dot-panel,natel-plotly-panel,savantly-heatmap-panel,grafana-piechart-panel,grafana-worldmap-panel" && \
    GF_PATHS_PLUGINS="/var/lib/grafana/plugins" && \
    IFS="," && \
    for plugin in ${GF_INSTALL_PLUGINS}; do \
    grafana-cli --pluginsDir "${GF_PATHS_PLUGINS}" plugins install ${plugin}; \
    done && \
    mkdir -p /var/lib/grafana/plugins/flant-statusmap-panel && \
    mv /plugin-dist /var/lib/grafana/plugins/flant-statusmap-panel/ && \
    chmod +r /etc/grafana/grafana.ini
EXPOSE 3000
COPY ./run.sh /run.sh
WORKDIR /
ENTRYPOINT ["/run.sh"]
