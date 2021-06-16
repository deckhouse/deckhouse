ARG BASE_ALPINE
ARG BASE_DEBIAN
ARG BASE_GOLANG_BUSTER

# This Dockerfile is based on Dockerfile from https://github.com/grafana/grafana.
# Changes:
# - Source are not available in current directory, so download them using an intermediate image.
# - Use debian to build go binaries.
# - Use debian for final stage.
# - Install bundled plugins in final stage.

# Step 1. Download sources and apply patches.
# It will fail fast on problems with future versions.
FROM $BASE_ALPINE as src-files
WORKDIR /usr/src/app
RUN apk add --no-cache patch
RUN wget https://github.com/grafana/grafana/archive/v7.4.2.tar.gz -O - | tar -xz  --strip-components=1
# Extra vars for prometheus datasource.
COPY ./feat_prometheus_extra_vars.patch .
RUN patch -p1 < ./feat_prometheus_extra_vars.patch
# Fix heatmap render.
COPY ./fix_heatmap_thin_bars_on_ff.patch .
RUN patch -p1 < ./fix_heatmap_thin_bars_on_ff.patch
# Set useful version. There are tabs in patch, so -l is used.
COPY ./build_go.patch .
RUN patch -p1 -l < ./build_go.patch
# Copy bundled plugins at start.
COPY ./run_sh.patch .
RUN patch -p1 -l < ./run_sh.patch


# Step 2. Build frontend.
# No optimizations, just copy whole source code.
FROM node:14.15.1-alpine3.12@sha256:ba8661b07004a0aedfbba12b08ac78e3f02c5ee3b7a68871b41d0e7ea5711c00 as js-builder
COPY --from=src-files /usr/src/app /usr/src/app
WORKDIR /usr/src/app
RUN yarn install --pure-lockfile --no-progress
ENV NODE_ENV production
RUN yarn build


# Step 3. Build backend.
# No optimizations, just copy whole source code.
FROM $BASE_GOLANG_BUSTER as go-builder
#RUN apk add --no-cache gcc g++
COPY --from=src-files /usr/src/app /usr/src/app
WORKDIR /usr/src/app/
#WORKDIR $GOPATH/src/github.com/grafana/grafana
#COPY go.mod go.sum ./
RUN go mod verify
#COPY pkg pkg
#COPY build.go package.json ./
RUN go run build.go build


# Step 4. Build the final image.
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

RUN mkdir -p "$GF_PATHS_PROVISIONING/datasources" \
             "$GF_PATHS_PROVISIONING/dashboards" \
             "$GF_PATHS_PROVISIONING/notifiers" \
             "$GF_PATHS_PROVISIONING/plugins" \
             "$GF_PATHS_LOGS" \
             "$GF_PATHS_PLUGINS" \
             "$GF_PATHS_DATA" && \
    cp "$GF_PATHS_HOME/conf/sample.ini" "$GF_PATHS_CONFIG" && \
    cp "$GF_PATHS_HOME/conf/ldap.toml" /etc/grafana/ldap.toml

COPY --from=go-builder /usr/src/app/bin/linux-amd64/grafana-server /usr/src/app/bin/linux-amd64/grafana-cli ./bin/
COPY --from=js-builder /usr/src/app/public ./public
COPY --from=js-builder /usr/src/app/tools ./tools

# Install bundled plugins.
RUN BUNDLED_PLUGINS="grafana-image-renderer,petrslavotinek-carpetplot-panel,vonage-status-panel,btplc-status-dot-panel,natel-plotly-panel,savantly-heatmap-panel,grafana-piechart-panel,grafana-worldmap-panel" && \
    IFS="," && \
    for plugin in ${BUNDLED_PLUGINS}; do \
      grafana-cli --pluginsDir "${GF_PATHS_PLUGINS}" plugins install ${plugin}; \
    done && \
    chmod +r /etc/grafana/grafana.ini
# Save path with bundled plugins.
ENV BUNDLED_PLUGINS_PATH="${GF_PATHS_PLUGINS}"

# Install the Statusmap.
RUN STATUSMAP_VERSION=0.4.1 && \
    STATUSMAP_ARCHIVE=flant-statusmap-panel-${STATUSMAP_VERSION}.zip && \
    curl -LSsO https://github.com/flant/grafana-statusmap/releases/download/v${STATUSMAP_VERSION}/${STATUSMAP_ARCHIVE} && \
    unzip ${STATUSMAP_ARCHIVE} -d "${GF_PATHS_PLUGINS}" && \
    rm ${STATUSMAP_ARCHIVE}

EXPOSE 3000
# Patched entrypoint script.
COPY --from=src-files /usr/src/app/packaging/docker/run.sh /run.sh
ENTRYPOINT ["/run.sh"]
