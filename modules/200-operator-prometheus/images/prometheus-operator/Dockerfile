ARG BASE_DISTROLESS
FROM quay.io/prometheus-operator/prometheus-operator:v0.62.0@sha256:83974739969e6f5f1672d7efb3fb710717beda26a5b69f1f9b9d89a8d4f00bcf as artifact

FROM $BASE_DISTROLESS

ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

RUN apk add --no-cache git
RUN git clone --depth 1 --branch v0.62.0 ${SOURCE_REPO}/prometheus-operator/prometheus-operator.git /prometheus-operator
WORKDIR /prometheus-operator/

RUN go build -ldflags="-s -w" -o operator cmd/operator/main.go && \
    chown -R 64535:64535 /prometheus-operator/ && \
    chmod 0700 /prometheus-operator/operator

FROM $BASE_DISTROLESS

COPY --from=artifact /prometheus-operator/operator /bin/
ENTRYPOINT ["/bin/operator"]
