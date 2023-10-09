ARG BASE_DISTROLESS
ARG BASE_GOLANG_19_ALPINE
FROM $BASE_GOLANG_19_ALPINE as artifact
WORKDIR /src/
RUN apk add --no-cache git mercurial && \
    wget https://codeload.github.com/kubernetes/autoscaler/tar.gz/vertical-pod-autoscaler/v0.14.0 -O - | tar -xz --strip-components=1 -C /src/ && \
    cd vertical-pod-autoscaler/ && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o updater pkg/updater/main.go

FROM $BASE_DISTROLESS
COPY --from=artifact /src/vertical-pod-autoscaler/updater /updater
ENTRYPOINT ["/updater"]
CMD ["--v=4", "--stderrthreshold=info"]
