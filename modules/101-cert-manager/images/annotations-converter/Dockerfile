ARG BASE_ALPINE
ARG BASE_GOLANG_16_ALPINE
FROM $BASE_GOLANG_16_ALPINE as artifact
WORKDIR /src/
COPY main.go go.mod go.sum /src/
RUN apk add --no-cache git && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o annotations-converter main.go

FROM $BASE_ALPINE
COPY --from=artifact /src/annotations-converter /annotations-converter
USER nobody
ENTRYPOINT [ "/annotations-converter" ]
