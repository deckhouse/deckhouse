ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
ARG BASE_DEBIAN

FROM $BASE_ALPINE as cloud-provider-vsphere
WORKDIR /src/
RUN apk add --no-cache patch
RUN wget https://github.com/flant/cloud-provider-vsphere/archive/v0.2.1.tar.gz -O - | tar -xz --strip-components=1 -C /src
COPY cloud-provider-vsphere-patches/001-support-fcd-disk-resize.patch /src
RUN patch -p1 < 001-support-fcd-disk-resize.patch

FROM $BASE_GOLANG_ALPINE as csi
COPY --from=cloud-provider-vsphere /src /cloud-provider-vsphere
WORKDIR /src/
RUN apk add --no-cache git mercurial patch
# TODO: switch to git tags once they become available in the upstream repository
RUN wget https://github.com/flant/vsphere-csi-driver/archive/6189afc2522d83a96d3857110c61478710110347.tar.gz -O - | tar -xz --strip-components=1 -C /src
COPY 001-csi-metrics-and-volume-expansion.patch /src
RUN patch -p1 < 001-csi-metrics-and-volume-expansion.patch
# pay attention when changing upstream version, k8s.io/cloud-provider-vsphere may change, which will lead to unexpected results
RUN go mod edit -replace=k8s.io/cloud-provider-vsphere=/cloud-provider-vsphere
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o vsphere-csi cmd/vsphere-csi/main.go

# support every standard Linux disk/mount utility so that CSI components won't complain
FROM $BASE_DEBIAN
COPY --from=csi /src/vsphere-csi /bin/vsphere-csi
ENTRYPOINT [ "/bin/vsphere-csi" ]
