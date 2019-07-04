FROM alpine:3.10
COPY --from=k8s.gcr.io/coredns:1.5.0 /coredns  /coredns
COPY src/health.sh /
RUN apk add --no-cache bind-tools
ENTRYPOINT ["/coredns"]
