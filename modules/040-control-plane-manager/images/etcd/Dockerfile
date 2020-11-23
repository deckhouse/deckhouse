ARG BASE_ALPINE

FROM $BASE_ALPINE

COPY --from=k8s.gcr.io/etcd:3.4.13-0 /usr/local/bin/etcd /usr/local/bin/etcd
COPY --from=k8s.gcr.io/etcd:3.4.13-0 /usr/local/bin/etcdctl /usr/local/bin/etcdctl
COPY --from=k8s.gcr.io/pause:3.1 /pause /pause

RUN mkdir -p /var/etcd/
RUN mkdir -p /var/lib/etcd/

RUN echo 'hosts: files mdns4_minimal [NOTFOUND=return] dns mdns4' >> /etc/nsswitch.conf

ENTRYPOINT [ "/usr/local/bin/etcd" ]
