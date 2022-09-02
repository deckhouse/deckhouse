ARG BASE_ALPINE
ARG BASE_GOLANG_18_ALPINE
FROM $BASE_GOLANG_18_ALPINE as artifact
WORKDIR /src
COPY src /src/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o psi-monitor psi-monitor.go

FROM $BASE_ALPINE
COPY --from=artifact /src/psi-monitor /
ENTRYPOINT [ "/psi-monitor" ]
