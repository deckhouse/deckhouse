ARG BASE_ALPINE
ARG BASE_NODE_16_ALPINE
ARG BASE_GOLANG_17_BUSTER
ARG BASE_GOLANG_20_ALPINE
ARG BASE_UBUNTU
ARG BASE_DISTROLESS
ARG GRAFANA_VERSION="8.5.13"
ARG STATUSMAP_VERSION="0.5.1"
ARG BUNDLED_PLUGINS="petrslavotinek-carpetplot-panel,vonage-status-panel,btplc-status-dot-panel,natel-plotly-panel,savantly-heatmap-panel,grafana-piechart-panel,grafana-worldmap-panel"

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
RUN apk add --no-cache patch git

ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}

ARG GRAFANA_VERSION

RUN git clone --depth 1 --branch v${GRAFANA_VERSION} ${SOURCE_REPO}/grafana/grafana.git .
# Extra '__interval_*' vars for prometheus datasource.
COPY ./patches/feat_prometheus_extra_vars.patch .
RUN patch -p1 < ./feat_prometheus_extra_vars.patch
# Fix heatmap render: constant bucket widths for fast-forward datasource.
COPY ./patches/fix_heatmap_thin_bars_on_ff.patch .
RUN patch -p1 < ./fix_heatmap_thin_bars_on_ff.patch
# Set more useful version than 'dev'. There are tabs in patch, so -l is used.
COPY patches/build_go.patch .
RUN patch -p1 -l < ./build_go.patch
# Patch to copy bundled plugins at start from ro directory to rw.
COPY ./patches/run_sh.patch .
RUN patch -p1 < ./run_sh.patch

# Download flant-statusmap-panel plugin.
ARG STATUSMAP_VERSION
RUN git clone --depth 1 --branch v${STATUSMAP_VERSION} ${SOURCE_REPO}/flant/grafana-statusmap.git /grafana-statusmap

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

FROM $BASE_NODE_16_ALPINE as grafana-statusmap
COPY --from=src-files /grafana-statusmap /grafana-statusmap
WORKDIR /grafana-statusmap
RUN apk --no-cache add git
RUN yarn install
ENV NODE_ENV production
RUN yarn build

# ===================================================
# Step 3. Backend binaries.
# Difference from original:
# - No COPY actions: copy whole source code at start.
# - WORKDIR $GOPATH/src/github.com/grafana/grafana is not needed to build with go modules.
# - Add 'make gen-go' to fix 'cli.go:163:12: undefined: server.Initialize'
# - Use debian: see step 4 for details.
FROM $BASE_GOLANG_17_BUSTER as go-builder
ARG GRAFANA_VERSION
ENV GRAFANA_VERSION=${GRAFANA_VERSION}
RUN apt-get update && \
    apt-get -y --no-install-recommends install git gcc musl musl-tools
COPY --from=src-files /usr/src/app /usr/src/app
WORKDIR /usr/src/app/

ARG GOPROXY
ENV GOPROXY=${GOPROXY} \
    CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=amd64 \
    CC=/usr/bin/musl-gcc

RUN make gen-go
RUN go build -ldflags -w -ldflags "-X main.version=${GRAFANA_VERSION} -linkmode external -extldflags -static" -tags netgo -o ./bin/linux-amd64/grafana-server ./pkg/cmd/grafana-server
RUN go build -ldflags -w -ldflags "-X main.version=${GRAFANA_VERSION} -linkmode external -extldflags -static" -tags netgo -o ./bin/linux-amd64/grafana-cli ./pkg/cmd/grafana-cli

FROM $BASE_GOLANG_20_ALPINE as entrypoint

COPY /entrypoint/ /app/
WORKDIR /app/

ARG GOPROXY
ENV GOPROXY=${GOPROXY} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

RUN go build -ldflags="-s -w" -o entrypoint main.go
RUN chown -R 64535:64535 /app/ && \
    chmod 0700 /app/entrypoint

# ===================================================
# Step 4. Final image.
# Difference from original:
# - No LABEL
# - No GF_UID, GF_GID
# - No USER, no adding user and group, no chmod
# - Install additional plugins
# - Install statusmap plugin
# - Use debian because of grafana-image-renderer plugin: plugin_start_linux_amd64 binary is not working in musl environment.
FROM $BASE_UBUNTU as grafana-distr

ENV PATH="/usr/share/grafana/bin:$PATH" \
    GF_PATHS_CONFIG="/etc/grafana/grafana.ini" \
    GF_PATHS_DATA="/var/lib/grafana" \
    GF_PATHS_HOME="/usr/share/grafana" \
    GF_PATHS_LOGS="/var/log/grafana" \
    GF_PATHS_PLUGINS="/usr/share/grafana/plugins-bundled" \
    GF_PATHS_PROVISIONING="/etc/grafana/provisioning"

WORKDIR $GF_PATHS_HOME

RUN apt-get update && \
    apt-get -y --no-install-recommends install libfontconfig curl ca-certificates openssl unzip git && \
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

# Copy flant-statusmap-panel plugin.
COPY --from=grafana-statusmap /grafana-statusmap/dist/ ${GF_PATHS_PLUGINS}/flant-statusmap-panel/

# Home Dashboard
COPY ./grafana_home_dashboard.json /usr/share/grafana/public/dashboards/grafana_home_dashboard.json
COPY ./web/ /usr/share/grafana/public/img/

RUN chown -R 64535:64535 /usr/share/grafana && \
    chown -R 64535:64535 /etc/grafana && \
    chown -R 64535:64535 /var/lib/grafana && \
    chown -R 64535:64535 /var/log/grafana

RUN chmod 0700 ./bin/grafana-server && \
    chmod 0700 ./bin/grafana-cli

FROM $BASE_DISTROLESS

COPY --from=entrypoint /app/entrypoint /usr/local/bin/
COPY --from=grafana-distr /usr/share/grafana/ /usr/share/grafana/
COPY --from=grafana-distr /etc/grafana/ /etc/grafana/
COPY --from=grafana-distr /var/lib/grafana/ /var/lib/grafana/
COPY --from=grafana-distr /var/log/grafana/ /var/log/grafana/

ENV PATH="/usr/share/grafana/bin:$PATH" \
    GF_PATHS_CONFIG="/etc/grafana/grafana.ini" \
    GF_PATHS_DATA="/var/lib/grafana" \
    GF_PATHS_HOME="/usr/share/grafana" \
    GF_PATHS_LOGS="/var/log/grafana" \
    GF_PATHS_PLUGINS="/usr/share/grafana/plugins-bundled" \
    GF_PATHS_PROVISIONING="/etc/grafana/provisioning"

ENV BUNDLED_PLUGINS_PATH="${GF_PATHS_PLUGINS}"

WORKDIR $GF_PATHS_HOME

EXPOSE 3000

ENTRYPOINT ["entrypoint"]
