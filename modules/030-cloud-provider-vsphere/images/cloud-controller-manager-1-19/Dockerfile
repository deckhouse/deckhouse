ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE
WORKDIR /src/
RUN apk add --no-cache git mercurial patch make
RUN wget https://github.com/kubernetes/cloud-provider-vsphere/archive/6d76e17cad08d583b8ff6fc5c80f119807ed0f57.tar.gz -O - | tar -xz --strip-components=1 -C /src/
COPY 001-base-on-kubernetes-1-19-3.patch \
    002-network-name-discovery-logic.patch \
    003-folder-path-option.patch \
    004-dont-initialize-node-without-internal-ip.patch \
    005-ignore-static-nodes.patch \
    006-extend-fcd-component-for-csi.patch \
    007-instance-exists-by-providerid-bugfix.patch /src/
RUN patch -p1 < 001-base-on-kubernetes-1-19-3.patch && \
    patch -p1 < 002-network-name-discovery-logic.patch && \
    patch -p1 < 003-folder-path-option.patch && \
    patch -p1 < 004-dont-initialize-node-without-internal-ip.patch && \
    patch -p1 < 005-ignore-static-nodes.patch && \
    patch -p1 < 006-extend-fcd-component-for-csi.patch && \
    patch -p1 < 007-instance-exists-by-providerid-bugfix.patch
RUN make build

FROM $BASE_ALPINE
RUN apk add --no-cache ca-certificates
COPY --from=0 /src/.build/bin/vsphere-cloud-controller-manager.linux_amd64 /bin/vsphere-cloud-controller-manager
ENTRYPOINT [ "/bin/vsphere-cloud-controller-manager" ]
