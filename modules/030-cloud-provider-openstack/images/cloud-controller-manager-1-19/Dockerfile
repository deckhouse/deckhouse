ARG BASE_GOLANG_BUSTER
ARG BASE_DEBIAN
FROM $BASE_GOLANG_BUSTER
WORKDIR /src/
COPY 001-ignore-static-nodes.patch /src/
RUN apt-get update && apt-get install -y --no-install-recommends git mercurial patch
RUN wget https://github.com/kubernetes/cloud-provider-openstack/archive/v1.19.2.tar.gz -O - | tar -xz --strip-components=1 -C /src/
RUN patch -p1 < 001-ignore-static-nodes.patch
RUN make openstack-cloud-controller-manager

# support every standard Linux disk/mount utility so that CSI components won't complain
FROM $BASE_DEBIAN
RUN apt-get update && apt-get install -y ca-certificates e2fsprogs mount xfsprogs udev
COPY --from=0 /src/openstack-cloud-controller-manager-amd64 /bin/openstack-cloud-controller-manager
ENTRYPOINT [ "/bin/openstack-cloud-controller-manager" ]
