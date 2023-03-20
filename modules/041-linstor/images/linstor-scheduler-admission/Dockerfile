ARG BASE_UBUNTU
ARG BASE_GOLANG_19_BULLSEYE

FROM $BASE_GOLANG_19_BULLSEYE as builder
ARG LINSTOR_SCHEDULER_EXTENDER_GITREPO=https://github.com/piraeusdatastore/linstor-scheduler-extender
ARG LINSTOR_SCHEDULER_EXTENDER_VERSION=0.3.1

RUN git clone ${LINSTOR_SCHEDULER_EXTENDER_GITREPO} /usr/local/go/linstor-scheduler-extender \
 && cd /usr/local/go/linstor-scheduler-extender \
 && git reset --hard v${LINSTOR_SCHEDULER_EXTENDER_VERSION} \
 && cd cmd/linstor-scheduler-admission \
 && go build -ldflags="-X github.com/piraeusdatastore/linstor-scheduler-extender/pkg/consts.Version=v${LINSTOR_SCHEDULER_EXTENDER_VERSION}" \
 && mv ./linstor-scheduler-admission /

FROM $BASE_UBUNTU
COPY --from=builder /linstor-scheduler-admission /
USER nonroot
ENTRYPOINT ["/linstor-scheduler-admission"]
