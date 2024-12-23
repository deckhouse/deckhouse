ARG BASE_DISTROLESS
ARG BASE_GOLANG_23_ALPINE
FROM $BASE_GOLANG_23_ALPINE as artifact
ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /src/
COPY / /src/
RUN go build -ldflags="-s -w" -o multitenancy-manager ./cmd/main.go

RUN chown 64535:64535 multitenancy-manager
RUN chmod 0700 multitenancy-manager

FROM $BASE_DISTROLESS

COPY --from=artifact /src/multitenancy-manager /multitenancy-manager
COPY --from=artifact /src/templates /templates
COPY --from=artifact /src/default /default

EXPOSE 9090
EXPOSE 9443

CMD [ "/multitenancy-manager" ]