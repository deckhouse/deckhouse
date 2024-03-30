ARG BASE_DISTROLESS
ARG BASE_GOLANG_21_ALPINE

FROM $BASE_GOLANG_21_ALPINE as artifact

ARG GOPROXY
ENV GOPROXY=${GOPROXY}

WORKDIR /src
ADD src /src
ENV GOARCH=amd64
RUN CGO_ENABLED=0 go build -a -ldflags '-s -w -extldflags "-static"' -o /image-availability-exporter main.go && \
    chown -R 64535:64535 /image-availability-exporter && \
    chmod 0700 /image-availability-exporter

FROM $BASE_DISTROLESS
COPY --from=artifact /image-availability-exporter /image-availability-exporter
ENTRYPOINT ["/image-availability-exporter"]
