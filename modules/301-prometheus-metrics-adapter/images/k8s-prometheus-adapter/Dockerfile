ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE_DEV

FROM $BASE_GOLANG_20_ALPINE_DEV AS build
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}
ARG GOPROXY
ENV GOPROXY=${GOPROXY}
WORKDIR /src
RUN git clone --depth 1 --branch v0.9.1 ${SOURCE_REPO}/kubernetes-sigs/prometheus-adapter.git .

RUN go get golang.org/x/net@v0.17.0 \
    && go get github.com/prometheus/client_golang@v1.11.1 \
    && go get gopkg.in/yaml.v3@v3.0.1 \
    && go get github.com/emicklei/go-restful@v2.16.0+incompatible \
    && go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w' -o adapter ./cmd/adapter/adapter.go

RUN chown 64535:64535 adapter
RUN chmod 0700 adapter

FROM $BASE_DISTROLESS
COPY --from=build /src/adapter /adapter
ENTRYPOINT [ "/adapter" ]
