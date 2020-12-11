ARG BASE_UBUNTU
FROM $BASE_UBUNTU

COPY --from=k8s.gcr.io/pause:3.1 /pause /pause

ARG KUBERNETES_VERSION=v1.19.5

ENV DEBIAN_FRONTEND=noninteractive \
    container=docker \
    KUBERNETES_DOWNLOAD_ROOT=https://storage.googleapis.com/kubernetes-release/release/${KUBERNETES_VERSION}/bin/linux/amd64 \
    KUBERNETES_COMPONENT=kube-controller-manager

RUN apt-get update \
    && apt install ca-certificates apt-transport-https  gnupg gnupg2 gnupg1 curl -y \
    && curl -sL 'https://download.ceph.com/keys/release.asc' | apt-key add - \
    && echo deb https://download.ceph.com/debian-nautilus/ bionic main | tee /etc/apt/sources.list.d/ceph.list \
    && apt-get update \
    && apt-get install -y ceph-common \
    && touch /etc/ceph/ceph.conf /etc/ceph/ceph.keyring \
    && curl -L ${KUBERNETES_DOWNLOAD_ROOT}/${KUBERNETES_COMPONENT} -o /usr/bin/${KUBERNETES_COMPONENT} \
    && chmod +x /usr/bin/${KUBERNETES_COMPONENT} \
    && apt-get purge -y --auto-remove curl apt-transport-https  gnupg gnupg2 gnupg1 \
    && rm -rf /var/lib/apt/lists/*
