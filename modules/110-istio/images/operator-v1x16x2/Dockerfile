# Based on https://github.com/istio/istio/blob/1.16.2/operator/docker/Dockerfile.operator
ARG BASE_UBUNTU
FROM docker.io/istio/operator:1.16.2@sha256:59f69b479576b9d90fbac96724f94248975cb62b212095ef21cc904511a923f9 as artifact

FROM $BASE_UBUNTU

# install operator binary
COPY --from=artifact /usr/local/bin/operator /usr/local/bin/

# add operator manifests
COPY --from=artifact /var/lib/istio/manifests/ /var/lib/istio/manifests/

USER 1337:1337

ENTRYPOINT ["/usr/local/bin/operator"]
