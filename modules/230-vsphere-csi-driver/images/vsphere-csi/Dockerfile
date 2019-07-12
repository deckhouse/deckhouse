FROM golang:1.12
WORKDIR /src/
COPY csi-metrics.patch /src/
RUN apt update && apt-get install -y --no-install-recommends git mercurial patch && \
    git clone https://github.com/kubernetes-sigs/vsphere-csi-driver.git && \
    cd /src/vsphere-csi-driver && \
    patch -p1 < /src/csi-metrics.patch && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o vsphere-csi cmd/vsphere-csi/main.go

# support every standard Linux disk/mount utility so that CSI components won't complain
FROM debian:stretch
COPY --from=0 /src/vsphere-csi-driver/vsphere-csi /bin/vsphere-csi
ENTRYPOINT [ "/bin/vsphere-csi" ]
