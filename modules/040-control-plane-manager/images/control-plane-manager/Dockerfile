FROM golang:1.13.12-buster as kubeadm_builder
WORKDIR /src/
RUN apt update
RUN apt install rsync patch -y

COPY patch /patch

RUN mkdir /src/kubernetes-1.16 && wget https://github.com/kubernetes/kubernetes/archive/v1.16.15.tar.gz -O - | tar -xz --strip-components=1 -C /src/kubernetes-1.16
RUN cd kubernetes-1.16 && \
  patch -p1 < /patch/v1.16/01-add-retry-to-etcd-operations.patch && \
  patch -p1 < /patch/v1.16/02-handle-multiple-members.patch && \
  patch -p1 < /patch/v1.16/03-increase-timeouts.patch && \
  patch -p1 < /patch/v1.16/04-remove-cluster-status-dependency.patch && \
  patch -p1 < /patch/v1.16/05-upgrade-path-from-cluster-status-to-annotations.patch && \
  patch -p1 < /patch/v1.16/06-plus.patch && \
  patch -p1 < /patch/v1.16/07-local-init-configuration.patch && \
  patch -p1 < /patch/v1.16/08-preserve-order-of-user-specified-apiserver.patch && \
  make all WHAT=cmd/kubeadm && mv _output/bin/kubeadm /kubeadm-1.16

RUN mkdir /src/kubernetes-1.17 && wget https://github.com/kubernetes/kubernetes/archive/v1.17.11.tar.gz -O - | tar -xz --strip-components=1 -C /src/kubernetes-1.17
RUN cd kubernetes-1.17 && \
  patch -p1 < /patch/v1.17/04-remove-cluster-status-dependency.patch && \
  patch -p1 < /patch/v1.17/05-upgrade-path-from-cluster-status-to-annotations.patch && \
  patch -p1 < /patch/v1.17/06-plus.patch && \
  patch -p1 < /patch/v1.17/07-local-init-configuration.patch && \
  make all WHAT=cmd/kubeadm && mv _output/bin/kubeadm /kubeadm-1.17

RUN mkdir /src/kubernetes-1.18 && wget https://github.com/kubernetes/kubernetes/archive/v1.18.8.tar.gz -O - | tar -xz --strip-components=1 -C /src/kubernetes-1.18
RUN cd kubernetes-1.18 && \
  patch -p1 < /patch/v1.18/07-local-init-configuration.patch && \
  make all WHAT=cmd/kubeadm && mv _output/bin/kubeadm /kubeadm-1.18

FROM alpine:3.11
RUN apk add --no-cache jq curl bash gettext grep
RUN curl -L https://storage.googleapis.com/kubernetes-release/release/v1.16.15/bin/linux/amd64/kubectl -o /usr/local/bin/kubectl && \
  chmod +x /usr/local/bin/kubectl
RUN curl -L https://github.com/cloudflare/cfssl/releases/download/v1.4.1/cfssl-certinfo_1.4.1_linux_amd64 -o /usr/local/bin/cfssl-certinfo && \
  chmod +x /usr/local/bin/cfssl-certinfo
RUN curl -L https://github.com/cloudflare/cfssl/releases/download/v1.4.1/cfssl_1.4.1_linux_amd64 -o /usr/local/bin/cfssl && \
  chmod +x /usr/local/bin/cfssl
ADD control-plane-manager /control-plane-manager

COPY --from=kubeadm_builder /kubeadm-1.16 /usr/local/bin/
COPY --from=kubeadm_builder /kubeadm-1.17 /usr/local/bin/
COPY --from=kubeadm_builder /kubeadm-1.18 /usr/local/bin/

COPY --from=k8s.gcr.io/pause:3.1 /pause /pause

ENTRYPOINT [ "/control-plane-manager" ]
