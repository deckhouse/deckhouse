ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE

FROM $BASE_GOLANG_20_ALPINE AS build
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}
ARG GOPROXY
ENV GOPROXY=${GOPROXY}
WORKDIR /src
RUN apk add --update --no-cache git
RUN git clone --depth 1 --branch v0.9.1 ${SOURCE_REPO}/kubernetes-sigs/prometheus-adapter.git .

RUN go get -u golang.org/x/net@v0.17.0 \
    && go get -u github.com/prometheus/client_golang@v1.11.1 \
    && go get -u gopkg.in/yaml.v3@v3.0.1 \
    && go get -u github.com/emicklei/go-restful@v2.16.0 \
    && go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w' -o adapter ./cmd/adapter/adapter.go

RUN chown 64535:64535 adapter
RUN chmod 0700 adapter

FROM $BASE_DISTROLESS
COPY --from=build /src/adapter /adapter
ENTRYPOINT [ "/adapter" ]
