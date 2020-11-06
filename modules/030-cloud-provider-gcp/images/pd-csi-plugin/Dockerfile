ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE as builder
WORKDIR /go/src/sigs.k8s.io/gcp-compute-persistent-disk-csi-driver
RUN wget https://github.com/kubernetes-sigs/gcp-compute-persistent-disk-csi-driver/archive/v0.7.0.tar.gz -O - | tar -xz --strip-components=1 -C /go/src/sigs.k8s.io/gcp-compute-persistent-disk-csi-driver/
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-X main.vendorVersion='"${TAG:-latest}"' -extldflags "-static"' -o bin/gce-pd-csi-driver ./cmd/

FROM gcr.io/google-containers/debian-base-amd64:v2.0.0 as base
RUN clean-install udev

FROM gcr.io/google-containers/debian-base-amd64:v2.0.0
COPY --from=builder /go/src/sigs.k8s.io/gcp-compute-persistent-disk-csi-driver/bin/gce-pd-csi-driver /gce-pd-csi-driver
RUN clean-install util-linux e2fsprogs mount ca-certificates udev xfsprogs
COPY --from=base /lib/udev/scsi_id /lib/udev_containerized/scsi_id
ENTRYPOINT [ "/gce-pd-csi-driver" ]
