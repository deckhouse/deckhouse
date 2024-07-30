ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE

FROM $BASE_GOLANG_20_ALPINE as artifact

ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /src/user-authz-webhook
COPY main.go go.mod go.sum /src/user-authz-webhook/
COPY cache /src/user-authz-webhook/cache/
COPY web /src/user-authz-webhook/web/
COPY cmd /src/user-authz-webhook/cmd/

RUN go test ./...
RUN go build -ldflags="-s -w" -o user-authz-webhook main.go
RUN go build -ldflags="-s -w" -o healthcheck ./cmd/healthcheck/main.go

RUN chown 64535:64535 user-authz-webhook healthcheck
RUN chmod 0700 user-authz-webhook healthcheck

FROM $BASE_DISTROLESS
COPY --from=artifact /src/user-authz-webhook/user-authz-webhook /user-authz-webhook
COPY --from=artifact /src/user-authz-webhook/healthcheck /healthcheck
ENTRYPOINT [ "/user-authz-webhook" ]
