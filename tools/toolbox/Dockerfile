FROM alpine:3.16

ENV GOLANGCI_VERSION=1.46.2 \
    TRIVY_VERSION=0.28.1 \
    PROMTOOL_VERSION=2.37.0 \
    GATOR_VERSION=3.9.0 \
    CRANE_VERSION=0.10.0 \
    WERF_VERSION=1.2 \
    TRDL_VERSION=0.6.3

RUN apk add --no-cache make go curl bash yq jq jq-dev oniguruma-dev git

# Install crane deps for update-patchversion script.
RUN curl -sSfL https://github.com/google/go-containerregistry/releases/download/v${CRANE_VERSION}/go-containerregistry_linux_x86_64.tar.gz | tar -xzf - -C /usr/local/bin crane

RUN curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin/trivy v${TRIVY_VERSION}

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | bash -s -- v${GOLANGCI_VERSION}

RUN curl -sSfL https://github.com/prometheus/prometheus/releases/download/v${PROMTOOL_VERSION}/prometheus-${PROMTOOL_VERSION}.linux-amd64.tar.gz -o - | tar zxf - -C /usr/local/bin --strip=1 prometheus-${PROMTOOL_VERSION}.linux-amd64/promtool

RUN curl -sSfL https://github.com/open-policy-agent/gatekeeper/releases/download/v${GATOR_VERSION}/gator-v${GATOR_VERSION}-linux-amd64.tar.gz -o - | tar zxf - -C /usr/local/bin gator

# Install trdl for installing werf
RUN curl -sSfL https://tuf.trdl.dev/targets/releases/${TRDL_VERSION}/linux-amd64/bin/trdl -o /usr/local/bin/trdl \
 && chmod +x /usr/local/bin/trdl

# Install werf for images-tags generator.
RUN trdl add werf https://tuf.werf.io 1 b7ff6bcbe598e072a86d595a3621924c8612c7e6dc6a82e919abe89707d7e3f468e616b5635630680dd1e98fc362ae5051728406700e6274c5ed1ad92bea52a2 \
 && trdl --no-self-update=true update werf ${WERF_VERSION} stable \
 && ln -s $(trdl bin-path werf ${WERF_VERSION} stable)/werf /usr/local/bin/werf

# Suppress safe.directory warnings
RUN git config --global --add safe.directory '*'
