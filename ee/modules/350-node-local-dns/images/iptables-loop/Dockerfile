FROM alpine:3.17.2 as apk_extractor

WORKDIR /workdir

RUN apk update
RUN apk fetch -R -o . busybox bash iptables inotify-tools alpine-baselayout-data

RUN mkdir /apk && for apk in *.apk; do tar -xzvf "$apk" -C /apk; done

COPY iptables-wrapper-installer.sh /
RUN /iptables-wrapper-installer.sh

FROM scratch

COPY --from=apk_extractor /apk/ /

COPY iptables-loop.sh /

ENTRYPOINT ["/iptables-loop.sh"]
