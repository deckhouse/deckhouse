ARG BASE_DISTROLESS
ARG BASE_GOLANG_18_ALPINE

FROM $BASE_GOLANG_18_ALPINE as artifact

RUN apk add --no-cache make git
RUN git clone --depth 1 --branch v0.62.0 https://github.com/prometheus-operator/prometheus-operator.git /prometheus-operator
WORKDIR /prometheus-operator/

RUN make prometheus-config-reloader
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o prometheus-config-reloader cmd/prometheus-config-reloader/main.go

FROM $BASE_DISTROLESS

COPY --from=artifact /prometheus-operator/prometheus-config-reloader /bin/

ENTRYPOINT ["/bin/prometheus-config-reloader"]
