FROM alpine:3.10
COPY --from=k8s.gcr.io/coredns:1.5.0 /coredns  /coredns
COPY start.sh /
RUN apk add --no-cache bind-tools bash curl
ENTRYPOINT ["/coredns"]
