ARG BASE_DISTROLESS
FROM quay.io/prometheus-operator/prometheus-operator:v0.62.0@sha256:83974739969e6f5f1672d7efb3fb710717beda26a5b69f1f9b9d89a8d4f00bcf as artifact

FROM $BASE_DISTROLESS

RUN apk add --no-cache make git
RUN git clone --depth 1 --branch v0.62.0 https://github.com/prometheus-operator/prometheus-operator.git /prometheus-operator
WORKDIR /prometheus-operator/

COPY local_probe.patch ./
RUN git apply local_probe.patch

RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o operator cmd/operator/main.go

FROM $BASE_DISTROLESS

COPY --from=artifact /prometheus-operator/operator /bin/

ENTRYPOINT ["/bin/operator"]
