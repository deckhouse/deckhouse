FROM cgr.dev/chainguard/apko:0.7.1 as apk_extractor

WORKDIR /workdir

COPY apko.yaml .

RUN apko build --arch x86_64 apko.yaml image:latest image.tar
RUN tar -xvf image.tar
RUN mkdir /rootfs && tar -xzvf *.tar.gz -C /rootfs

COPY iptables-wrapper-installer.sh /
RUN /iptables-wrapper-installer.sh

FROM scratch

COPY --from=apk_extractor /rootfs/ /
COPY --from=coredns/coredns:1.9.3@sha256:bdb36ee882c13135669cfc2bb91c808a33926ad1a411fee07bd2dc344bb8f782 /coredns /coredns

COPY start.sh /
COPY readiness.sh /
COPY liveness.sh /

ENTRYPOINT ["/coredns"]
