FROM golang:1.9.2-alpine3.7
WORKDIR /go/src/github.com/kubernetes-incubator/descheduler
RUN apk add --update git make
RUN git clone https://github.com/kubernetes-incubator/descheduler.git . \
  && git checkout v0.8.0
RUN make

FROM alpine:3.9
COPY --from=0  /go/src/github.com/kubernetes-incubator/descheduler/_output/bin/descheduler /bin/descheduler
CMD ["/bin/descheduler", "--help"]

