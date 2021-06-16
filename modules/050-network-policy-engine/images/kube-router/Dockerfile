# Based on https://github.com/cloudnativelabs/kube-router/blob/master/Dockerfile
ARG BASE_ALPINE
FROM docker.io/cloudnativelabs/kube-router:v0.3.2@sha256:12459f482a57f496737378921dcc57a50bbf414acaf92d31a46513ef5c116c92 as artifact

FROM $BASE_ALPINE
RUN apk add --no-cache \
      iptables \
      ip6tables \
      ipset \
      iproute2 \
      ipvsadm \
      conntrack-tools \
      curl \
      bash && \
    mkdir -p /var/lib/gobgp && \
    mkdir -p /usr/local/share/bash-completion && \
    curl -L -o /usr/local/share/bash-completion/bash-completion \
        https://raw.githubusercontent.com/scop/bash-completion/master/bash_completion
COPY --from=artifact /root/.bashrc /root
COPY --from=artifact /root/.profile /root
COPY --from=artifact /root/.vimrc /root
COPY --from=artifact /etc/motd-kube-router.sh /etc
COPY --from=artifact /usr/local/bin/kube-router /usr/local/bin/gobgp /usr/local/bin/
COPY iptables-wrapper-installer.sh /
RUN /iptables-wrapper-installer.sh --no-sanity-check
RUN echo "hosts: files dns" > /etc/nsswitch.conf

WORKDIR /root
ENTRYPOINT ["/usr/local/bin/kube-router"]
