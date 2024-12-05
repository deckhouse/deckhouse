ARG BASE_DISTROLESS
ARG BASE_GOLANG_22_BULLSEYE

FROM $BASE_GOLANG_22_BULLSEYE AS build
# Also check operator image version and admission-policy-engine module trivy-provider image "github.com/aquasecurity/trivy" library version
ARG TRIVY_VERSION=v0.55.0-flant
ARG TRIVY_DB_VERSION=flant-v2
ARG GOPROXY
ARG SOURCE_REPO
ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /src
RUN mkdir -p ~/.ssh && echo "StrictHostKeyChecking accept-new" > ~/.ssh/config
RUN --mount=type=ssh git clone --depth 1 --branch ${TRIVY_DB_VERSION} ${SOURCE_REPO}/aquasecurity/trivy-db.git
RUN --mount=type=ssh git clone --depth 1 --branch ${TRIVY_VERSION} ${SOURCE_REPO}/aquasecurity/trivy.git
RUN cd trivy && \
    go build -ldflags '-s -w -extldflags "-static"' -o trivy ./cmd/trivy/main.go

RUN chown 64535:64535 trivy
RUN chmod 0700 trivy

FROM $BASE_DISTROLESS
COPY --from=build /src/trivy/trivy /usr/local/bin/
ENTRYPOINT [ "/usr/local/bin/trivy" ]
