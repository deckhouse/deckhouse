ARG BASE_GOLANG_19_ALPINE

FROM $BASE_GOLANG_19_ALPINE as artifact
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o resolv-watcher main.go


FROM scratch
COPY --from=artifact /src/resolv-watcher /
ENTRYPOINT [ "/resolv-watcher" ]
