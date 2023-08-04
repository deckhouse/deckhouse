ARG BASE_GOLANG_19_BULLSEYE
ARG BASE_DISTROLESS

FROM $BASE_GOLANG_19_BULLSEYE as artifact
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

ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}

RUN mkdir /prometheus && cd /prometheus \
  && git clone -b "${PROMETHEUS_VERSION}" --single-branch ${SOURCE_REPO}/prometheus/prometheus
WORKDIR /prometheus/prometheus

RUN go mod download

COPY sample_limit_annotation.patch ./
COPY successfully_sent_metric.patch ./

RUN git apply sample_limit_annotation.patch && \
  git apply successfully_sent_metric.patch && \
  make build

RUN mkdir -p /consoles && \
  cp /prometheus/prometheus/consoles/* /consoles && \
  cp /prometheus/prometheus/console_libraries/* /consoles

RUN mkdir -p /etc && \
  cp /prometheus/prometheus/documentation/examples/prometheus.yml /etc && \
  cp /prometheus/prometheus/console_libraries/* /etc

RUN mkdir -p /prometheus

FROM $BASE_DISTROLESS
COPY --from=artifact /prometheus/prometheus/prometheus                             /bin/prometheus
COPY --from=artifact /prometheus/prometheus/promtool                               /bin/promtool
# COPY --from=artifact /prometheus/prometheus/documentation/examples/prometheus.yml  /etc/prometheus/prometheus.yml
COPY --from=artifact /prometheus/prometheus/console_libraries/                     /usr/share/prometheus/console_libraries/
# COPY --from=artifact /prometheus/prometheus/consoles/                              /usr/share/prometheus/consoles/
COPY --from=artifact /consoles/                                                    /usr/share/prometheus/consoles/
COPY --from=artifact /etc/                                                         /etc/prometheus/
COPY --from=artifact /prometheus/                                                  /prometheus/

# RUN apk --no-cache add curl
# RUN ln -s /usr/share/prometheus/console_libraries /usr/share/prometheus/consoles/ /etc/prometheus/
# RUN mkdir -p /prometheus && \
    # chown -R 65534:2000 etc/prometheus /prometheus

USER       65534
EXPOSE     9090
VOLUME     [ "/prometheus" ]
WORKDIR    /prometheus
ENTRYPOINT [ "/bin/prometheus" ]
CMD        [ "--config.file=/etc/prometheus/prometheus.yml", \
             "--storage.tsdb.path=/prometheus", \
             "--web.console.libraries=/usr/share/prometheus/console_libraries", \
             "--web.console.templates=/usr/share/prometheus/consoles" ]
