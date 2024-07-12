ARG BASE_DISTROLESS
ARG BASE_GOLANG_19_ALPINE_DEV
FROM $BASE_GOLANG_19_ALPINE_DEV as artifact

WORKDIR /src/
COPY entrypoint/ /src/
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o entrypoint main.go

FROM $BASE_DISTROLESS

COPY --from=artifact /src/entrypoint /
ENTRYPOINT [ "/entrypoint" ]
