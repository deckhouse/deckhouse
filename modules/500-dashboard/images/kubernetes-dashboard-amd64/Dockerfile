FROM golang:1.11 AS builder
# Install Java, Node.js and Chromium browser. Go is already installed.
# A small tweak, apt-get update is already run by the nodejs setup script, so there's no need to run it again.
RUN curl -sL https://deb.nodesource.com/setup_9.x | bash - \
  && apt-get install -y --no-install-recommends \
        openjdk-8-jre \
        nodejs \
        patch \
        chromium \
        git \
        && rm -rf /var/lib/apt/lists/* \
        && apt-get clean

# Set environment variable for JavaScript tests.
ENV CHROME_BIN=/usr/bin/chromium

# Current directory is always /dashboard.
COPY probe.patch /
WORKDIR /dashboard
RUN wget https://github.com/kubernetes/dashboard/archive/v1.10.1.tar.gz -O - | tar -xz --strip-components=1 -C /dashboard && \
  patch -p1 < /probe.patch

# Install dependencies. This will take a while.
RUN npm install -g npm@latest gulp
RUN npm install --unsafe-perm

RUN gulp build

FROM scratch

COPY --from=builder /dashboard/dist/amd64/locale_conf.json /locale_conf.json
COPY --from=builder /dashboard/dist/amd64/dashboard /dashboard
COPY --from=builder /dashboard/dist/amd64/public /public

EXPOSE 9090 8443
ENTRYPOINT ["/dashboard", "--insecure-bind-address=0.0.0.0", "--bind-address=0.0.0.0"]
