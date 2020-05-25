FROM k8s.gcr.io/kube-controller-manager:v1.17.6

COPY --from=k8s.gcr.io/pause:3.1 /pause /pause

RUN set -x \
    && apt-get update \
    && apt install wget gnupg apt-transport-https ca-certificates -y \
    && wget -q -O- 'https://download.ceph.com/keys/release.asc' | apt-key add - \
    && echo deb https://download.ceph.com/debian-nautilus/ stretch main | tee /etc/apt/sources.list.d/ceph.list \
    && apt-get update \
    && apt-get install -y \
        ceph-common \
    && touch /etc/ceph/ceph.conf /etc/ceph/ceph.keyring \
    && rm -rf /var/lib/apt/lists/*
