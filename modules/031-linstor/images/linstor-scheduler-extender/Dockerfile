ARG BASE_DEBIAN
ARG BASE_GOLANG_17_BUSTER

FROM $BASE_GOLANG_17_BUSTER as builder
ARG LINSTOR_SCHEDULER_EXTENDER_GITREPO=https://github.com/piraeusdatastore/linstor-scheduler-extender
ARG LINSTOR_SCHEDULER_EXTENDER_VERSION=0.2.0

RUN git clone ${LINSTOR_SCHEDULER_EXTENDER_GITREPO} /usr/local/go/linstor-scheduler-extender \
 && cd /usr/local/go/linstor-scheduler-extender \
 && git reset --hard v${LINSTOR_SCHEDULER_EXTENDER_VERSION} \
 && cd cmd/linstor-scheduler-extender \
 && go build -ldflags="-X github.com/piraeusdatastore/linstor-scheduler-extender/pkg/consts.Version=v${LINSTOR_SCHEDULER_EXTENDER_VERSION}" \
 && mv ./linstor-scheduler-extender /

FROM $BASE_DEBIAN
COPY --from=builder /linstor-scheduler-extender /

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

ENTRYPOINT ["/linstor-scheduler-extender"]
