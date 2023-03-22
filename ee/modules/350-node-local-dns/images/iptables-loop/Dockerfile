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

COPY iptables-loop.sh /

ENTRYPOINT ["/iptables-loop.sh"]
