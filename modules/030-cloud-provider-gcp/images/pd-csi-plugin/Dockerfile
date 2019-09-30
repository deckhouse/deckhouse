FROM golang:1.12-alpine
WORKDIR /go/src/sigs.k8s.io/gcp-compute-persistent-disk-csi-driver
RUN wget https://github.com/kubernetes-sigs/gcp-compute-persistent-disk-csi-driver/archive/95946b4cd68d1d65fa9786753b4ec1ba81af2184.tar.gz -O - | tar -xz --strip-components=1 -C /go/src/sigs.k8s.io/gcp-compute-persistent-disk-csi-driver/
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-X main.vendorVersion='"${TAG:-latest}"' -extldflags "-static"' -o bin/gce-pd-csi-driver ./cmd/

FROM gcr.io/google-containers/debian-base-amd64:v1.0.0
COPY --from=0 /go/src/sigs.k8s.io/gcp-compute-persistent-disk-csi-driver/bin/gce-pd-csi-driver /gce-pd-csi-driver
RUN clean-install util-linux e2fsprogs mount ca-certificates udev
ENTRYPOINT [ "/gce-pd-csi-driver" ]
