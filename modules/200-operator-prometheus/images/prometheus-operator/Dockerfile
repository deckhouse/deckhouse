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

RUN apk add --no-cache git
RUN echo ${SOURCE_REPO}
RUN git clone --depth 1 --branch v0.68.0 ${SOURCE_REPO}/prometheus-operator/prometheus-operator.git /prometheus-operator
WORKDIR /prometheus-operator/

RUN go build -ldflags="-s -w" -o operator cmd/operator/main.go && \
    chown -R 64535:64535 /prometheus-operator/ && \
    chmod 0700 /prometheus-operator/operator

FROM $BASE_DISTROLESS

COPY --from=artifact /prometheus-operator/operator /bin/
ENTRYPOINT ["/bin/operator"]
