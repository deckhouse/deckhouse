ARG BASE_ALPINE
FROM $BASE_ALPINE
RUN apk add --no-cache curl iproute2
COPY face-slapper.sh /
ENTRYPOINT ["/face-slapper.sh"]
