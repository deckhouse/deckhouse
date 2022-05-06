ARG BASE_ALPINE
ARG BASE_NODE_16_ALPINE
ARG BASE_GOLANG_17_BUSTER
ARG BASE_DEBIAN
ARG GRAFANA_VERSION="8.5.2"
ARG STATUSMAP_VERSION="0.5.1"
ARG BUNDLED_PLUGINS="grafana-image-renderer,petrslavotinek-carpetplot-panel,vonage-status-panel,btplc-status-dot-panel,natel-plotly-panel,savantly-heatmap-panel,grafana-piechart-panel,grafana-worldmap-panel"

# This Dockerfile is based on Dockerfile from https://github.com/grafana/grafana/blob/v8.5.2/Dockerfile
# Changes:
# - Source files are not available in the current directory.
#   Archive is downloaded and patched using an intermediate image.
# - Install bundled plugins in final stage.

# ===================================================
# Step 1. Download sources and apply patches.
# It will fail fast on problems with future versions.
FROM $BASE_ALPINE as src-files
WORKDIR /usr/src/app
RUN apk add --no-cache patch
ARG GRAFANA_VERSION
RUN wget https://github.com/grafana/grafana/archive/v${GRAFANA_VERSION}.tar.gz -O - | tar -xz  --strip-components=1
# Extra '__interval_*' vars for prometheus datasource.
COPY ./patches/feat_prometheus_extra_vars.patch .
RUN patch -p1 < ./feat_prometheus_extra_vars.patch
# Fix heatmap render: constant bucket widths for fast-forward datasource.
COPY ./patches/fix_heatmap_thin_bars_on_ff.patch .
RUN patch -p1 < ./fix_heatmap_thin_bars_on_ff.patch
# Set more useful version than 'dev'. There are tabs in patch, so -l is used.
COPY ./patches/build_go.patch .
RUN patch -p1 -l < ./build_go.patch
# Patch to copy bundled plugins at start from ro directory to rw.
COPY ./patches/run_sh.patch .
RUN patch -p1 < ./run_sh.patch


# ===================================================
# Step 2. Frontend.
# Difference from original:
# - No COPY actions: copy whole source code at start.
# - NODE_OPTIONS to fix "JavaScript heap out of memory" error
FROM $BASE_NODE_16_ALPINE as js-builder
COPY --from=src-files /usr/src/app /usr/src/app
WORKDIR /usr/src/app
RUN apk --no-cache add git
RUN yarn install
ENV NODE_ENV production
ENV NODE_OPTIONS "--max_old_space_size=8000"
RUN yarn build


# ===================================================
# Step 3. Backend binaries.
# Difference from original:
# - No COPY actions: copy whole source code at start.
# - WORKDIR $GOPATH/src/github.com/grafana/grafana is not needed to build with go modules.
# - Add 'make gen-go' to fix 'cli.go:163:12: undefined: server.Initialize'
# - Use debian: see step 4 for details.
FROM $BASE_GOLANG_17_BUSTER as go-builder
COPY --from=src-files /usr/src/app /usr/src/app
WORKDIR /usr/src/app/
RUN make gen-go
RUN go run build.go build


# ===================================================
# Step 4. Final image.
# Difference from original:
# - No LABEL
# - No GF_UID, GF_GID
# - No USER, no adding user and group, no chmod
# - Install additional plugins
# - Install statusmap plugin
# - Use debian because of grafana-image-renderer plugin: plugin_start_linux_amd64 binary is not working in musl environment.
FROM $BASE_DEBIAN

ENV PATH="/usr/share/grafana/bin:$PATH" \
    GF_PATHS_CONFIG="/etc/grafana/grafana.ini" \
    GF_PATHS_DATA="/var/lib/grafana" \
    GF_PATHS_HOME="/usr/share/grafana" \
    GF_PATHS_LOGS="/var/log/grafana" \
    GF_PATHS_PLUGINS="/var/lib/grafana/plugins" \
    GF_PATHS_PROVISIONING="/etc/grafana/provisioning"

WORKDIR $GF_PATHS_HOME

RUN apt-get update && \
    apt-get -y --no-install-recommends install libfontconfig curl ca-certificates tzdata openssl unzip && \
    apt-get clean && \
    apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/*

COPY --from=src-files /usr/src/app/conf ./conf

RUN mkdir -p "$GF_PATHS_HOME/.aws" && \
    mkdir -p "$GF_PATHS_PROVISIONING/datasources" \
             "$GF_PATHS_PROVISIONING/dashboards" \
             "$GF_PATHS_PROVISIONING/notifiers" \
             "$GF_PATHS_PROVISIONING/plugins" \
             "$GF_PATHS_PROVISIONING/access-control" \
             "$GF_PATHS_LOGS" \
             "$GF_PATHS_PLUGINS" \
             "$GF_PATHS_DATA" && \
    cp "$GF_PATHS_HOME/conf/sample.ini" "$GF_PATHS_CONFIG" && \
    cp "$GF_PATHS_HOME/conf/ldap.toml" /etc/grafana/ldap.toml && \
    chmod -R 777 "$GF_PATHS_DATA" "$GF_PATHS_HOME/.aws" "$GF_PATHS_LOGS" "$GF_PATHS_PLUGINS" "$GF_PATHS_PROVISIONING"

COPY --from=go-builder /usr/src/app/bin/*/grafana-server /usr/src/app/bin/*/grafana-cli ./bin/
COPY --from=js-builder /usr/src/app/public ./public
COPY --from=js-builder /usr/src/app/tools ./tools

# Install bundled plugins.
ARG BUNDLED_PLUGINS
RUN echo Add bundled plugins: ${BUNDLED_PLUGINS} && \
    IFS="," && \
    for plugin in ${BUNDLED_PLUGINS}; do \
      grafana-cli --pluginsDir "${GF_PATHS_PLUGINS}" plugins install ${plugin}; \
    done && \
    chmod +r /etc/grafana/grafana.ini
# Save path with bundled plugins.
ENV BUNDLED_PLUGINS_PATH="${GF_PATHS_PLUGINS}"

# Download flant-statusmap-panel plugin.
ARG STATUSMAP_VERSION
RUN echo "Fetch flant-statusmap-panel v${STATUSMAP_VERSION}" && \
    STATUSMAP_ARCHIVE=flant-statusmap-panel-${STATUSMAP_VERSION}.zip && \
    curl -LSsO https://github.com/flant/grafana-statusmap/releases/download/v${STATUSMAP_VERSION}/${STATUSMAP_ARCHIVE} && \
    unzip ${STATUSMAP_ARCHIVE} -d "${GF_PATHS_PLUGINS}" && \
    rm ${STATUSMAP_ARCHIVE}

# Home Dashboard
COPY ./grafana_home_dashboard.json /usr/share/grafana/public/dashboards/grafana_home_dashboard.json
COPY ./web/ /usr/share/grafana/public/img/

EXPOSE 3000
# Patched entrypoint script.
COPY --from=src-files /usr/src/app/packaging/docker/run.sh /run.sh
ENTRYPOINT ["/run.sh"]
