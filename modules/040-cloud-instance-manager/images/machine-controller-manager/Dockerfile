FROM golang:1.13-alpine
WORKDIR /src/
ARG CI_JOB_TOKEN
RUN apk add --no-cache git mercurial make bash tzdata
RUN git clone https://gitlab-ci-token:${CI_JOB_TOKEN}@fox.flant.com/sys/deckhouse-mcm.git /src/ && git checkout 0.23.0-flant.3
RUN make build-local

FROM alpine:3.10

COPY --from=0 /src/bin/machine-controller-manager /machine-controller-manager
WORKDIR /
ENTRYPOINT ["/machine-controller-manager"]
