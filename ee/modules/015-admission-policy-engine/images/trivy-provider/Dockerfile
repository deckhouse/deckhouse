ARG BASE_GOLANG_22_BULLSEYE
ARG BASE_DISTROLESS

FROM $BASE_GOLANG_22_BULLSEYE as artifact
ARG TRIVY_VERSION=v0.55.0-flant
ARG TRIVY_DB_VERSION=flant-v2
ARG GOPROXY
ARG SOURCE_REPO
ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /src/
COPY main.go go.mod go.sum /src/
COPY validators /src/validators/
COPY web /src/web/

RUN git clone --depth 1 --branch ${TRIVY_DB_VERSION} ${SOURCE_REPO}/aquasecurity/trivy-db.git && \
    git clone --depth 1 --branch ${TRIVY_VERSION} ${SOURCE_REPO}/aquasecurity/trivy.git && \
    go work init && go work use trivy && go work use trivy-db && go work use . && \
    go build -ldflags="-s -w" -o trivy-provider main.go


FROM $BASE_DISTROLESS
COPY --from=artifact /src/trivy-provider /bin/trivy-provider
ENTRYPOINT [ "/bin/trivy-provider" ]
