ARG BASE_GOLANG_BUSTER
ARG BASE_DEBIAN
FROM $BASE_GOLANG_BUSTER
WORKDIR /src/
RUN apt-get update && apt-get install -y --no-install-recommends git mercurial patch && \
    wget https://github.com/kubernetes/cloud-provider-openstack/archive/v1.19.0.tar.gz -O - | tar -xz --strip-components=1 -C /src/ && \
    make cinder-csi-plugin

# support every standard Linux disk/mount utility so that CSI components won't complain
FROM $BASE_DEBIAN
RUN apt-get update && apt-get install -y ca-certificates e2fsprogs mount xfsprogs udev
COPY --from=0 /src/cinder-csi-plugin /bin/cinder-csi-plugin
ENTRYPOINT [ "/bin/cinder-csi-plugin" ]
