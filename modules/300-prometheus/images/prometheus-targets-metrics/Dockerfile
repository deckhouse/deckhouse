ARG BASE_GOLANG_19_ALPINE
ARG BASE_SCRATCH

FROM $BASE_GOLANG_19_ALPINE as artifact
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o prometheus-targets-metrics main.go


FROM $BASE_SCRATCH
COPY --from=artifact /src/prometheus-targets-metrics /
ENTRYPOINT [ "/prometheus-targets-metrics" ]
