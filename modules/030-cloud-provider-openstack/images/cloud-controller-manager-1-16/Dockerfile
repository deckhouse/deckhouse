ARG BASE_GOLANG_BUSTER
ARG BASE_DEBIAN
FROM $BASE_GOLANG_BUSTER
WORKDIR /src/
RUN apt-get update && apt-get install -y --no-install-recommends git mercurial patch && \
    wget https://github.com/flant/cloud-provider-openstack/archive/5dcb9bbd77a3595781e6a9fb366fdc205aaabf7c.tar.gz -O - | tar -xz --strip-components=1 -C /src/ && \
    make openstack-cloud-controller-manager

# support every standard Linux disk/mount utility so that CSI components won't complain
FROM $BASE_DEBIAN
RUN apt-get update && apt-get install -y ca-certificates e2fsprogs mount xfsprogs udev
COPY --from=0 /src/openstack-cloud-controller-manager-amd64 /bin/openstack-cloud-controller-manager
ENTRYPOINT [ "/bin/openstack-cloud-controller-manager" ]
