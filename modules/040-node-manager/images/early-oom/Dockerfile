ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE
FROM $BASE_GOLANG_20_ALPINE as artifact
ARG GOPROXY
ARG SOURCE_REPO
ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64
WORKDIR /src
COPY src /src/

RUN go build -ldflags="-w -s" -o psi-monitor psi-monitor.go

RUN chown 64535:64535 psi-monitor
RUN chmod 0700 psi-monitor

FROM $BASE_DISTROLESS
COPY --from=artifact /src/psi-monitor /
ENTRYPOINT [ "/psi-monitor" ]
