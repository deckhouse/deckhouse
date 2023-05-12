ARG BASE_GOLANG_19_ALPINE

FROM $BASE_GOLANG_19_ALPINE as builder
WORKDIR /src
# https://github.com/kubernetes-sigs/iptables-wrappers/commit/5792812d9e5a5bb7f22d79d557bbfeece253343d
RUN wget https://github.com/kubernetes-sigs/iptables-wrappers/tarball/5792812d9e5a5bb7f22d79d557bbfeece253343d -O - | tar -xz --strip-components=1 -C /src
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o iptables-wrapper main.go

FROM scratch
COPY --from=builder /src/iptables-wrapper /
