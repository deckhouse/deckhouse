ARG BASE_DISTROLESS
ARG BASE_GOLANG_17_ALPINE

FROM $BASE_GOLANG_17_ALPINE as artifact
WORKDIR /src
COPY src /src/

ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

RUN go build -ldflags="-w -s" -o check-kernel-version check-kernel-version.go && chown 64535:64535 check-kernel-version && chmod 0755 check-kernel-version

FROM $BASE_DISTROLESS
COPY --from=artifact /src/check-kernel-version /
ENTRYPOINT [ "/check-kernel-version" ]
