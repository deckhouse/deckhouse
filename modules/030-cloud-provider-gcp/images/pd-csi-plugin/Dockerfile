ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
ARG BASE_DEBIAN
FROM $BASE_GOLANG_ALPINE as artifact
WORKDIR /go/src/sigs.k8s.io/gcp-compute-persistent-disk-csi-driver
RUN wget https://github.com/kubernetes-sigs/gcp-compute-persistent-disk-csi-driver/archive/v1.3.3.tar.gz -O - | tar -xz --strip-components=1 -C /go/src/sigs.k8s.io/gcp-compute-persistent-disk-csi-driver/
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-X main.version='"${TAG:-latest}"' -extldflags "-static"' -o bin/gce-pd-csi-driver ./cmd/gce-pd-csi-driver/

FROM $BASE_DEBIAN as base
RUN apt-get update && apt-get install -y --no-install-recommends udev && apt-get clean -y && rm -rf /var/cache/debconf/* /var/lib/apt/lists/* /var/log/* /tmp/* /var/tmp/*

FROM $BASE_DEBIAN
COPY --from=artifact /go/src/sigs.k8s.io/gcp-compute-persistent-disk-csi-driver/bin/gce-pd-csi-driver /gce-pd-csi-driver
RUN apt-get update && apt-get install -y --no-install-recommends util-linux e2fsprogs mount ca-certificates udev xfsprogs && apt-get clean -y && rm -rf /var/cache/debconf/* /var/lib/apt/lists/* /var/log/* /tmp/* /var/tmp/*
COPY --from=base /lib/udev/scsi_id /lib/udev_containerized/scsi_id
ENTRYPOINT [ "/gce-pd-csi-driver" ]
