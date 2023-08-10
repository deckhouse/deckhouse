# Based on https://github.com/open-policy-agent/gatekeeper/blob/master/Dockerfile
ARG BASE_GOLANG_20_ALPINE
ARG BASE_DISTROLESS

FROM $BASE_GOLANG_20_ALPINE as build
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}
WORKDIR /src

RUN apk add --no-cache git
RUN git clone --depth 1 --branch v3.10.0 ${SOURCE_REPO}/open-policy-agent/gatekeeper.git .
RUN go build -mod vendor -ldflags="-s -w" -o gatekeeper

RUN chown 64535:64535 gatekeeper
RUN chmod 0700 gatekeeper

FROM $BASE_DISTROLESS
COPY --from=build /src/gatekeeper /bin/gatekeeper
ENTRYPOINT ["/bin/gatekeeper"]
