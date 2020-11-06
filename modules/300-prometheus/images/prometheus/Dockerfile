ARG BASE_GOLANG_BUSTER
FROM $BASE_GOLANG_BUSTER

RUN curl -sL https://deb.nodesource.com/setup_12.x | bash - &&  \
  apt install -y nodejs && \
  npm update -g npm && \
  npm install webpack -g && \
  npm config set registry http://registry.npmjs.org/ && \
  apt-key adv --fetch-keys http://dl.yarnpkg.com/debian/pubkey.gpg && \
  echo "deb https://dl.yarnpkg.com/debian/ stable main" > /etc/apt/sources.list.d/yarn.list && \
  apt update && apt install -y yarn && \
  apt install -y make bash git patch ca-certificates openssl openssh-client

RUN mkdir /prometheus && cd /prometheus \
  && git clone -b "v2.16.0" --single-branch https://github.com/prometheus/prometheus
WORKDIR /prometheus/prometheus
COPY sample_limit_annotation.patch ./
COPY successfully_sent_metric.patch ./

RUN patch -p1 < sample_limit_annotation.patch && \
  patch -p1 < successfully_sent_metric.patch && \
  make build

FROM quay.io/prometheus/busybox-linux-amd64:latest
COPY --from=0 /prometheus/prometheus/prometheus                             /bin/prometheus
COPY --from=0 /prometheus/prometheus/promtool                               /bin/promtool
COPY --from=0 /prometheus/prometheus/documentation/examples/prometheus.yml  /etc/prometheus/prometheus.yml
COPY --from=0 /prometheus/prometheus/console_libraries/                     /usr/share/prometheus/console_libraries/
COPY --from=0 /prometheus/prometheus/consoles/                              /usr/share/prometheus/consoles/

RUN ln -s /usr/share/prometheus/console_libraries /usr/share/prometheus/consoles/ /etc/prometheus/
RUN mkdir -p /prometheus && \
    chown -R nobody:nogroup etc/prometheus /prometheus

USER       nobody
EXPOSE     9090
VOLUME     [ "/prometheus" ]
WORKDIR    /prometheus
ENTRYPOINT [ "/bin/prometheus" ]
CMD        [ "--config.file=/etc/prometheus/prometheus.yml", \
             "--storage.tsdb.path=/prometheus", \
             "--web.console.libraries=/usr/share/prometheus/console_libraries", \
             "--web.console.templates=/usr/share/prometheus/consoles" ]
