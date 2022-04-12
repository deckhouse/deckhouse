ARG BASE_DEBIAN
ARG BASE_GOLANG_17_BUSTER

FROM $BASE_GOLANG_17_BUSTER as builder
ARG LINSTOR_WAIT_UNTIL_VERSION=0.1.1

COPY . /usr/local/go/linstor-pools-importer
RUN cd /usr/local/go/linstor-pools-importer \
 && go build \
 && mv ./linstor-pools-importer /

RUN git clone https://github.com/LINBIT/linstor-wait-until /usr/local/go/linstor-wait-until \
 && cd /usr/local/go/linstor-wait-until \
 && git reset --hard v${LINSTOR_WAIT_UNTIL_VERSION} \
 && go build \
 && mv ./linstor-wait-until /

FROM $BASE_DEBIAN
COPY --from=builder /linstor-pools-importer /linstor-wait-until /

# needs a newer lvm, debian bug #932433
RUN printf 'Package: *\nPin: release a=%s\nPin-Priority: %s\n\n' stable 700 testing 650 > /etc/apt/preferences \
 && echo 'deb http://deb.debian.org/debian/ bullseye main contrib' >> /etc/apt/sources.list \
 && apt-get update \
 && apt-get install -y -t bullseye lvm2 \
 && apt-get remove -y udev \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/* \
 && sed -i 's/udev_rules.*=.*/udev_rules=0/ ; s/udev_sync.*=.*/udev_sync=0/ ; s/obtain_device_list_from_udev.*=.*/obtain_device_list_from_udev=0/' /etc/lvm/lvm.conf \
 && sed -i '/^devices {/a global_filter = [ "r|^/dev/drbd|" ]' /etc/lvm/lvm.conf

ENTRYPOINT ["/linstor-pools-importer"]
