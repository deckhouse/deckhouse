ARG BASE_SCRATCH
ARG BASE_GOLANG_17_ALPINE
FROM $BASE_GOLANG_17_ALPINE as artifact
WORKDIR /src
COPY src /src/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o check-kernel-version check-kernel-version.go

FROM $BASE_SCRATCH
COPY --from=artifact /src/check-kernel-version /
ENTRYPOINT [ "/check-kernel-version" ]
