FROM alpine:3.11

RUN apk add --no-cache jq curl bash gettext grep
RUN curl -L https://storage.googleapis.com/kubernetes-release/release/v1.16.8/bin/linux/amd64/kubectl -o /usr/local/bin/kubectl && \
  chmod +x /usr/local/bin/kubectl
RUN curl -L https://storage.googleapis.com/kubernetes-release/release/v1.16.8/bin/linux/amd64/kubeadm -o /usr/local/bin/kubeadm && \
  chmod +x /usr/local/bin/kubeadm
RUN curl -L https://github.com/cloudflare/cfssl/releases/download/v1.4.1/cfssl-certinfo_1.4.1_linux_amd64 -o /usr/local/bin/cfssl-certinfo && \
  chmod +x /usr/local/bin/cfssl-certinfo
RUN curl -L https://github.com/cloudflare/cfssl/releases/download/v1.4.1/cfssl_1.4.1_linux_amd64 -o /usr/local/bin/cfssl && \
  chmod +x /usr/local/bin/cfssl
RUN mkdir etcd && curl -L https://github.com/etcd-io/etcd/releases/download/v3.3.10/etcd-v3.3.10-linux-amd64.tar.gz | tar -xz --strip-components=1 -C etcd && \
  cp etcd/etcdctl /usr/local/bin/ && rm -r etcd && chmod +x /usr/local/bin/etcdctl
ADD control-plane-manager /control-plane-manager

COPY --from=k8s.gcr.io/pause:3.1 /pause /pause

ENTRYPOINT [ "/control-plane-manager" ]
