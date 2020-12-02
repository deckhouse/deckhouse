ARG BASE_ALPINE
ARG BASE_DEBIAN
ARG BASE_GOLANG_BUSTER
FROM $BASE_GOLANG_BUSTER
WORKDIR /src/
COPY csi-metrics-and-volume-expansion.patch /
RUN apt update && apt-get install -y --no-install-recommends git mercurial patch && \
    wget https://github.com/kubernetes-sigs/vsphere-csi-driver/archive/6189afc2522d83a96d3857110c61478710110347.tar.gz -O - | tar -xz --strip-components=1 -C /src/ && \
    patch -p1 < /csi-metrics-and-volume-expansion.patch && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o vsphere-csi cmd/vsphere-csi/main.go

# support every standard Linux disk/mount utility so that CSI components won't complain
FROM $BASE_DEBIAN
COPY --from=0 /src/vsphere-csi /bin/vsphere-csi
ENTRYPOINT [ "/bin/vsphere-csi" ]
