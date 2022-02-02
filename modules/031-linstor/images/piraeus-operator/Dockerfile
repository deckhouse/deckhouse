ARG BASE_DEBIAN
ARG BASE_GOLANG_17_BUSTER

FROM $BASE_GOLANG_17_BUSTER as builder
ARG PIRAEUS_OPERATOR_VERSION=a09bf6d

# Fix conflicting names
COPY ./patches /patches

RUN git clone https://github.com/piraeusdatastore/piraeus-operator /usr/local/go/piraeus-operator \
 && cd /usr/local/go/piraeus-operator \
 && git reset --hard ${PIRAEUS_OPERATOR_VERSION} \
 && git apply /patches/*.patch \
 && cd cmd/manager \
 && go build \
 && mv ./manager /piraeus-operator

FROM $BASE_DEBIAN
COPY --from=builder /piraeus-operator /usr/bin/piraeus-operator
ENTRYPOINT ["/usr/bin/piraeus-operator"]
