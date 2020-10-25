ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE
WORKDIR /go/src/github.com/kubernetes-incubator/descheduler
RUN apk add --update git make
RUN git clone https://github.com/kubernetes-incubator/descheduler.git . \
  && git checkout v0.8.0
RUN make

FROM $BASE_ALPINE
COPY --from=0  /go/src/github.com/kubernetes-incubator/descheduler/_output/bin/descheduler /bin/descheduler
CMD ["/bin/descheduler", "--help"]

