ARG BASE_GOLANG_BUSTER
FROM $BASE_GOLANG_BUSTER
WORKDIR /src/
RUN apt-get update && apt-get install -y --no-install-recommends git mercurial patch && \
    wget https://github.com/kubernetes/cloud-provider-openstack/archive/b9e62747cd65c74c3be9a042d3dd58ddaf561cfd.tar.gz -O - | tar -xz --strip-components=1 -C /src/ && \
    make cinder-csi-plugin

# support every standard Linux disk/mount utility so that CSI components won't complain
FROM debian:buster
RUN apt-get update && apt-get install -y ca-certificates e2fsprogs mount xfsprogs udev
COPY --from=0 /src/cinder-csi-plugin /bin/cinder-csi-plugin
ENTRYPOINT [ "/bin/cinder-csi-plugin" ]
