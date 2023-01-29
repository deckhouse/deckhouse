ARG BASE_UBUNTU
ARG BASE_GOLANG_18_BULLSEYE

FROM $BASE_GOLANG_18_BULLSEYE as builder
ARG LINSTOR_AFFINITY_CONTROLLER_GITREPO=https://github.com/piraeusdatastore/linstor-affinity-controller
ARG LINSTOR_AFFINITY_CONTROLLER_VERSION=0.2.2
ARG LINSTOR_WAIT_UNTIL_GITREPO=https://github.com/LINBIT/linstor-wait-until
ARG LINSTOR_WAIT_UNTIL_VERSION=0.2.1

RUN git clone ${LINSTOR_WAIT_UNTIL_GITREPO} /usr/local/go/linstor-wait-until \
 && cd /usr/local/go/linstor-wait-until \
 && git reset --hard v${LINSTOR_WAIT_UNTIL_VERSION} \
 && go build \
 && mv ./linstor-wait-until /

RUN git clone ${LINSTOR_AFFINITY_CONTROLLER_GITREPO} /usr/local/go/linstor-affinity-controller \
 && cd /usr/local/go/linstor-affinity-controller \
 && git reset --hard v${LINSTOR_AFFINITY_CONTROLLER_VERSION} \
 && cd cmd/linstor-affinity-controller \
 && go build -ldflags="-X github.com/piraeusdatastore/linstor-affinity-controller/pkg/consts.Version=v${LINSTOR_SCHEDULER_EXTENDER_VERSION}" \
 && mv ./linstor-affinity-controller /

FROM $BASE_UBUNTU
COPY --from=builder /linstor-affinity-controller /linstor-wait-until /
ENTRYPOINT ["/linstor-affinity-controller"]
