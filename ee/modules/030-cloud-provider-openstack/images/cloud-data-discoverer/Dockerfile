ARG BASE_ALPINE
ARG BASE_GOLANG_19_ALPINE
FROM $BASE_GOLANG_19_ALPINE as artifact
WORKDIR /src
COPY . /src/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o discoverer main.go

FROM $BASE_ALPINE
COPY --from=artifact /src/discoverer /
ENTRYPOINT [ "/discoverer" ]
