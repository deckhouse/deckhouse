FROM golang:1.13-alpine
WORKDIR /src/
ARG CI_JOB_TOKEN
RUN apk add --no-cache git mercurial make bash tzdata
RUN git clone -b vsphere --single-branch https://gitlab-ci-token:${CI_JOB_TOKEN}@fox.flant.com/sys/deckhouse-mcm.git /src/ && git checkout adacaf8cde65a362201376a2fa9da6a3336c26be
RUN make build-local

FROM alpine:3.10

COPY --from=0 /src/bin/machine-controller-manager /machine-controller-manager
WORKDIR /
ENTRYPOINT ["/machine-controller-manager"]
