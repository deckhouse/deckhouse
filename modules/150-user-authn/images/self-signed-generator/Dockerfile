ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE
FROM $BASE_GOLANG_20_ALPINE as artifact
ARG GOPROXY
ENV GOPROXY=${GOPROXY}
WORKDIR /src
COPY src /src/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o self-signed-generator main.go

RUN chown 64535:64535 self-signed-generator
RUN chmod 0700 self-signed-generator

FROM $BASE_DISTROLESS
COPY --from=artifact /src/self-signed-generator /
ENTRYPOINT [ "/self-signed-generator" ]
