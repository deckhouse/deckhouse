ARG BASE_GOLANG_18_ALPINE
ARG BASE_DEBIAN

FROM $BASE_GOLANG_18_ALPINE as csi
WORKDIR /src/
RUN apk add --no-cache patch
RUN wget https://github.com/kubernetes-sigs/vsphere-csi-driver/archive/refs/tags/v2.7.0.tar.gz -O - | tar -xz --strip-components=1 -C /src
COPY patches/001-fixed-non-existing-node-error-handling-on-unpublishvolume.patch /src
RUN patch -p1 < 001-fixed-non-existing-node-error-handling-on-unpublishvolume.patch
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o vsphere-csi cmd/vsphere-csi/main.go

# support every standard Linux disk/mount utility so that CSI components won't complain
FROM $BASE_DEBIAN
COPY --from=csi /src/vsphere-csi /bin/vsphere-csi
ENTRYPOINT [ "/bin/vsphere-csi" ]
