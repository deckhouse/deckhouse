ARG BASE_GOLANG_19_ALPINE
ARG BASE_DISTROLESS

FROM $BASE_GOLANG_19_ALPINE as artifact
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o resolv-watcher main.go


FROM $BASE_DISTROLESS
COPY --from=artifact /src/resolv-watcher /
ENTRYPOINT [ "/resolv-watcher" ]
