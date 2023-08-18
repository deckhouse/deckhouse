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

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w' -o adapter ./cmd/adapter/adapter.go

RUN chown 64535:64535 adapter
RUN chmod 0700 adapter

FROM $BASE_DISTROLESS
COPY --from=build /src/adapter /adapter
ENTRYPOINT [ "/adapter" ]
