ARG BASE_GOLANG_ALPINE
ARG BASE_UBUNTU

FROM $BASE_GOLANG_ALPINE as go-builder
ARG KUBERNETES_VERSION=v1.19.5

WORKDIR /src/
COPY pdb-daemonset.patch /

ENV KUBE_GIT_VERSION_FILE=.kube-version
RUN echo "KUBE_GIT_VERSION='${KUBERNETES_VERSION}-flant.$(ls /*.patch | wc -l)'"   >> $KUBE_GIT_VERSION_FILE && \
    echo "KUBE_GIT_MAJOR='$(echo ${KUBERNETES_VERSION} | tr -d v | cut -d. -f 1)'" >> $KUBE_GIT_VERSION_FILE && \
    echo "KUBE_GIT_MINOR='$(echo ${KUBERNETES_VERSION} | tr -d v | cut -d. -f 2)'" >> $KUBE_GIT_VERSION_FILE && \
    echo "KUBE_GIT_COMMIT='0000000000000000000000000000000000000000'"              >> $KUBE_GIT_VERSION_FILE && \
    echo "KUBE_GIT_TREE_STATE='archive'"                                           >> $KUBE_GIT_VERSION_FILE

RUN apk add --no-cache make bash git mercurial patch rsync && \
    wget https://github.com/kubernetes/kubernetes/archive/${KUBERNETES_VERSION}.tar.gz -O - | tar -xz --strip-components=1 -C /src/ && \
    patch -p1 < /pdb-daemonset.patch && \
    make all WHAT=cmd/kube-controller-manager


FROM $BASE_UBUNTU
ENV DEBIAN_FRONTEND=noninteractive \
    container=docker

RUN apt-get update \
    && apt install ca-certificates apt-transport-https  gnupg gnupg2 gnupg1 curl -y \
    && curl -sL 'https://download.ceph.com/keys/release.asc' | apt-key add - \
    && echo deb https://download.ceph.com/debian-nautilus/ bionic main | tee /etc/apt/sources.list.d/ceph.list \
    && apt-get update \
    && apt-get install -y ceph-common \
    && touch /etc/ceph/ceph.conf /etc/ceph/ceph.keyring \
    && apt-get purge -y --auto-remove curl apt-transport-https  gnupg gnupg2 gnupg1 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=k8s.gcr.io/pause:3.1 /pause /pause
COPY --from=go-builder /src/_output/bin/kube-controller-manager /usr/bin/kube-controller-manager

ENTRYPOINT [ "/usr/bin/kube-controller-manager" ]
