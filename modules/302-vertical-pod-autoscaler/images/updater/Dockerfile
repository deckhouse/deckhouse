ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE as artifact
WORKDIR /src/
COPY patches/daemonset-eviction.patch ./
RUN apk add --no-cache git mercurial patch && \
    wget https://codeload.github.com/kubernetes/autoscaler/tar.gz/vertical-pod-autoscaler/v0.9.2 -O - | tar -xz --strip-components=1 -C /src/ && \
    patch -p1 < /src/daemonset-eviction.patch && \
    cd vertical-pod-autoscaler/ && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o updater pkg/updater/main.go

FROM $BASE_ALPINE
COPY --from=artifact /src/vertical-pod-autoscaler/updater /updater
ENTRYPOINT ["/updater"]
CMD ["--v=4", "--stderrthreshold=info"]
