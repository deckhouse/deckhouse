ARG BASE_GOLANG_19_BULLSEYE
ARG BASE_GOLANG_18_ALPINE
ARG BASE_ALPINE

FROM $BASE_GOLANG_18_ALPINE as promu_artifact

ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

RUN apk add --no-cache git && \
    git clone --depth 1 --branch v0.14.0  ${SOURCE_REPO}/prometheus/promu.git /promu
WORKDIR /promu/

RUN go build -ldflags="-s -w" -o promu ./main.go

FROM $BASE_GOLANG_19_BULLSEYE as artifact
COPY --from=promu_artifact /promu/promu /bin/promu

ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

ENV PROMETHEUS_VERSION=v2.44.0

RUN curl -sL https://deb.nodesource.com/setup_16.x | bash - &&  \
  apt install -y nodejs && \
  npm update -g npm && \
  npm install webpack -g && \
  npm config set registry http://registry.npmjs.org/ && \
  apt-key adv --fetch-keys http://dl.yarnpkg.com/debian/pubkey.gpg && \
  echo "deb https://dl.yarnpkg.com/debian/ stable main" > /etc/apt/sources.list.d/yarn.list && \
  apt update && apt install -y yarn

RUN apt install -y make bash git ca-certificates openssl openssh-client bzip2

RUN mkdir /prometheus && cd /prometheus \
  && git clone -b "${PROMETHEUS_VERSION}" --single-branch ${SOURCE_REPO}/prometheus/prometheus

WORKDIR /prometheus/prometheus/web/ui
RUN npm install && \
    npm run build

WORKDIR /prometheus/prometheus

RUN scripts/compress_assets.sh && \
go mod download

COPY sample_limit_annotation.patch ./
COPY successfully_sent_metric.patch ./

RUN git apply sample_limit_annotation.patch && \
  git apply successfully_sent_metric.patch && \
  go generate -tags plugins ./plugins && \
  /bin/promu build --prefix /prometheus/prometheus

RUN mkdir -p /consoles && \
  cp /prometheus/prometheus/consoles/* /consoles && \
  cp /prometheus/prometheus/console_libraries/* /consoles

RUN mkdir -p /etc && \
  cp /prometheus/prometheus/documentation/examples/prometheus.yml /etc && \
  cp /prometheus/prometheus/console_libraries/* /etc

RUN chown -R 64535:64535 /prometheus/ && \
    chown -R 64535:64535 /etc/ && \
    chown -R 64535:64535 /consoles/ && \
    chmod 0700 /prometheus/prometheus/prometheus && \
    chmod 0700 /prometheus/prometheus/promtool

RUN mkdir -p /consoles && \
  cp /prometheus/prometheus/consoles/* /consoles && \
  cp /prometheus/prometheus/console_libraries/* /consoles

RUN mkdir -p /etc && \
  cp /prometheus/prometheus/documentation/examples/prometheus.yml /etc && \
  cp /prometheus/prometheus/console_libraries/* /etc

RUN chown -R 64535:64535 /prometheus/ && \
    chown -R 64535:64535 /etc/ && \
    chown -R 64535:64535 /consoles/ && \
    chmod 0700 /prometheus/prometheus/prometheus && \
    chmod 0700 /prometheus/prometheus/promtool

FROM $BASE_ALPINE
COPY --from=artifact /prometheus/prometheus/prometheus                             /bin/prometheus
COPY --from=artifact /prometheus/prometheus/promtool                               /bin/promtool
COPY --from=artifact /prometheus/prometheus/console_libraries/                     /usr/share/prometheus/console_libraries/
COPY --from=artifact /consoles/                                                    /usr/share/prometheus/consoles/
COPY --from=artifact /etc/                                                         /etc/prometheus/
COPY --from=artifact /prometheus/                                                  /prometheus/

RUN apk add --no-cache curl

EXPOSE     9090
VOLUME     [ "/prometheus" ]
WORKDIR    /prometheus
ENTRYPOINT [ "/bin/prometheus" ]
CMD        [ "--config.file=/etc/prometheus/prometheus.yml", \
             "--storage.tsdb.path=/prometheus", \
             "--web.console.libraries=/usr/share/prometheus/console_libraries", \
             "--web.console.templates=/usr/share/prometheus/consoles" ]
