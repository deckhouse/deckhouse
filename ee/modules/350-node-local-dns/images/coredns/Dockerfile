FROM debian:bullseye-20230208 as deb_extractor

WORKDIR /workdir

ENV PACKAGES="bash curl iptables iproute2 dnsutils jq grep coreutils libpcre3"

RUN apt-get update

RUN apt-get download $(apt-cache depends --recurse --no-recommends --no-suggests \
        --no-conflicts --no-breaks --no-replaces --no-enhances \
        --no-pre-depends ${PACKAGES} | grep "^\w")

RUN  mkdir -p /dpkg/var/lib/dpkg/status.d/ && \
        for deb in *.deb; do \
            package_name=$(dpkg-deb -I ${deb} | awk '/^ Package: .*$/ {print $2}'); \
            echo "Process: ${package_name}"; \
            dpkg --ctrl-tarfile $deb | tar -Oxvf - ./control > /dpkg/var/lib/dpkg/status.d/${package_name}; \
            dpkg --extract $deb /dpkg || exit 10; \
        done

COPY iptables-wrapper-installer.sh /
RUN /iptables-wrapper-installer.sh

FROM gcr.io/distroless/static-debian11

COPY --from=deb_extractor /dpkg/ /
COPY --from=coredns/coredns:1.9.3@sha256:bdb36ee882c13135669cfc2bb91c808a33926ad1a411fee07bd2dc344bb8f782 /coredns /coredns

COPY start.sh /
COPY readiness.sh /
COPY liveness.sh /


ENTRYPOINT ["/coredns"]
