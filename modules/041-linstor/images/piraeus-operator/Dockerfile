ARG BASE_UBUNTU
ARG BASE_GOLANG_18_BULLSEYE

FROM $BASE_GOLANG_18_BULLSEYE as builder
ARG PIRAEUS_OPERATOR_GITREPO=https://github.com/piraeusdatastore/piraeus-operator
ARG PIRAEUS_OPERATOR_VERSION=1.10.1

# Copy patches
COPY ./patches /patches

RUN git clone ${PIRAEUS_OPERATOR_GITREPO} /usr/local/go/piraeus-operator \
 && cd /usr/local/go/piraeus-operator \
 && git reset --hard v${PIRAEUS_OPERATOR_VERSION} \
 && git apply /patches/*.patch \
 && cd cmd/manager \
 && go build \
 && mv ./manager /piraeus-operator

FROM $BASE_UBUNTU
COPY --from=builder /piraeus-operator /usr/bin/piraeus-operator
ENTRYPOINT ["/usr/bin/piraeus-operator"]
