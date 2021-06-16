ARG BASE_ALPINE
FROM flant/kube-rbac-proxy:v0.1.1@sha256:d8ce260d21190440b30fbe34c24824c8bef0f7afc9b4c3895823c11716e7a2ff as artifact

FROM $BASE_ALPINE
COPY --from=artifact /kube-rbac-proxy /kube-rbac-proxy

EXPOSE 8080

ADD entrypoint.sh /

ENTRYPOINT ["/entrypoint.sh"]
