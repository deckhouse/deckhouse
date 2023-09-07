ARG BASE_DISTROLESS
ARG BASE_GOLANG_19_ALPINE

FROM $BASE_GOLANG_19_ALPINE as builder
WORKDIR /src

ARG GOPROXY
ARG SOURCE_REPO

ENV GOPROXY=${GOPROXY} \
    SOURCE_REPO=${SOURCE_REPO} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# https://github.com/kubernetes-sigs/iptables-wrappers/commit/680003b3c6e93b471a59ecc9ae87a8f9054b82f3
RUN apk add --update --no-cache patch git && \
    git clone ${SOURCE_REPO}/kubernetes-sigs/iptables-wrappers/tarball/5792812d9e5a5bb7f22d79d557bbfeece253343d /src && \
    cd /src && git reset --hard 680003b3c6e93b471a59ecc9ae87a8f9054b82f3 && \
    go build -ldflags="-s -w" -o iptables-wrapper main.go && ё \
    chown 64535:64535 iptables-wrapper && ё \
    chmod 0755 iptables-wrapper

FROM $BASE_DISTROLESS
COPY --from=builder /src/iptables-wrapper /
