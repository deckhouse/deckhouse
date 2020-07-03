FROM golang:1.13.12-buster as kubeadm_builder
WORKDIR /src/
RUN apt update
RUN apt install rsync -y
RUN wget https://github.com/flant/kubernetes/archive/v1.16.10-flant.2.tar.gz -O - | tar -xz --strip-components=1 -C /src
RUN make all WHAT=cmd/kubeadm

FROM alpine:3.11

RUN apk add --no-cache jq curl bash gettext grep
RUN curl -L https://storage.googleapis.com/kubernetes-release/release/v1.16.10/bin/linux/amd64/kubectl -o /usr/local/bin/kubectl && \
  chmod +x /usr/local/bin/kubectl
RUN curl -L https://github.com/cloudflare/cfssl/releases/download/v1.4.1/cfssl-certinfo_1.4.1_linux_amd64 -o /usr/local/bin/cfssl-certinfo && \
  chmod +x /usr/local/bin/cfssl-certinfo
RUN curl -L https://github.com/cloudflare/cfssl/releases/download/v1.4.1/cfssl_1.4.1_linux_amd64 -o /usr/local/bin/cfssl && \
  chmod +x /usr/local/bin/cfssl
ADD control-plane-manager /control-plane-manager

COPY --from=kubeadm_builder /src/_output/bin/kubeadm /usr/local/bin/
COPY --from=k8s.gcr.io/pause:3.1 /pause /pause

ENTRYPOINT [ "/control-plane-manager" ]
