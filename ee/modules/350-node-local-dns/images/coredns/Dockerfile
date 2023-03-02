FROM alpine:3.17.2 as apk_extractor

WORKDIR /workdir

RUN apk update
RUN apk fetch -R -o . busybox bash curl iptables iproute2 bind-tools jq alpine-baselayout-data

RUN mkdir /apk && for apk in *.apk; do tar -xzvf "$apk" -C /apk; done

COPY iptables-wrapper-installer.sh /
RUN /iptables-wrapper-installer.sh

FROM scratch

COPY --from=apk_extractor /apk/ /
COPY --from=coredns/coredns:1.9.3@sha256:bdb36ee882c13135669cfc2bb91c808a33926ad1a411fee07bd2dc344bb8f782 /coredns /coredns

COPY start.sh /
COPY readiness.sh /
COPY liveness.sh /


ENTRYPOINT ["/coredns"]
