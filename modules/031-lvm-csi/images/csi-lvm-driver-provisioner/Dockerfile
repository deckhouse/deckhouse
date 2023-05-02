FROM golang:1.19-alpine as builder
RUN apk add make binutils git
WORKDIR /work
RUN git clone --depth 1 https://github.com/kvaps/csi-driver-lvm.git -b shared-lun --single-branch .
RUN make provisioner

FROM alpine:3.16
LABEL maintainers="Metal Authors"
LABEL description="LVM Driver"

RUN apk add lvm2 lvm2-extra e2fsprogs e2fsprogs-extra smartmontools nvme-cli util-linux device-mapper
COPY --from=builder /work/bin/csi-lvmplugin-provisioner /csi-lvmplugin-provisioner
USER root
ENTRYPOINT ["/csi-lvmplugin-provisioner"]
