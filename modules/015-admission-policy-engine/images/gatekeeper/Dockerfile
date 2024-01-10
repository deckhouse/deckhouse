# Based on https://github.com/open-policy-agent/gatekeeper/blob/master/Dockerfile
ARG BASE_GOLANG_20_ALPINE_DEV
ARG BASE_DISTROLESS

FROM $BASE_GOLANG_20_ALPINE_DEV as build

ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /src

RUN git clone --depth 1 --branch v3.13.0 ${SOURCE_REPO}/open-policy-agent/gatekeeper.git .
RUN go build -mod vendor -ldflags="-s -w" -o gatekeeper

RUN chown 64535:64535 gatekeeper
RUN chmod 0700 gatekeeper

FROM $BASE_DISTROLESS
COPY --from=build /src/gatekeeper /bin/gatekeeper
ENTRYPOINT ["/bin/gatekeeper"]
