ARG BASE_DEBIAN_BULLSEYE
ARG BASE_GOLANG_18_BULLSEYE

FROM $BASE_GOLANG_18_BULLSEYE as builder
ARG PIRAEUS_HA_CONTROLLER_GITREPO=https://github.com/piraeusdatastore/piraeus-ha-controller
ARG PIRAEUS_HA_CONTROLLER_VERSION=0.3.0
ARG LINSTOR_WAIT_UNTIL_GITREPO=https://github.com/LINBIT/linstor-wait-until
ARG LINSTOR_WAIT_UNTIL_VERSION=0.1.1

RUN git clone ${PIRAEUS_HA_CONTROLLER_GITREPO} /usr/local/go/piraeus-ha-controller \
 && cd /usr/local/go/piraeus-ha-controller \
 && git reset --hard v${PIRAEUS_HA_CONTROLLER_VERSION} \
 && cd cmd/piraeus-ha-controller \
 && go build -ldflags="-X github.com/piraeusdatastore/piraeus-ha-controller/pkg/consts.Version=v${PIRAEUS_HA_CONTROLLER_VERSION}" \
 && mv ./piraeus-ha-controller /

RUN git clone ${LINSTOR_WAIT_UNTIL_GITREPO} /usr/local/go/linstor-wait-until \
 && cd /usr/local/go/linstor-wait-until \
 && git reset --hard v${LINSTOR_WAIT_UNTIL_VERSION} \
 && go build \
 && mv ./linstor-wait-until /

FROM $BASE_DEBIAN_BULLSEYE
COPY --from=builder /piraeus-ha-controller /linstor-wait-until /
USER nonroot
ENTRYPOINT ["/piraeus-ha-controller"]
