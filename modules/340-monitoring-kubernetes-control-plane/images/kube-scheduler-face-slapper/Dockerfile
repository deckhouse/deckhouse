FROM alpine:3.11
RUN apk add --no-cache curl iproute2
COPY face-slapper.sh /
ENTRYPOINT ["/face-slapper.sh"]
