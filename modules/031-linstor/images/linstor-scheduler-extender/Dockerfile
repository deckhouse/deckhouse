ARG BASE_DEBIAN
ARG BASE_GOLANG_17_BUSTER

FROM $BASE_GOLANG_17_BUSTER as builder
ARG LINSTOR_SCHEDULER_EXTENDER_VERSION=0.1.0

RUN git clone https://github.com/deckhouse/linstor-scheduler-extender /usr/local/go/linstor-scheduler-extender \
 && cd /usr/local/go/linstor-scheduler-extender \
 && git reset --hard v${LINSTOR_SCHEDULER_EXTENDER_VERSION} \
 && cd cmd/linstor-scheduler-extender \
 && go build -ldflags="-X github.com/kvaps/linstor-scheduler-extender/pkg/consts.Version=v${LINSTOR_SCHEDULER_EXTENDER_VERSION}" \
 && mv ./linstor-scheduler-extender /

FROM $BASE_DEBIAN
COPY --from=builder /linstor-scheduler-extender /
ENTRYPOINT ["/linstor-scheduler-extender"]
