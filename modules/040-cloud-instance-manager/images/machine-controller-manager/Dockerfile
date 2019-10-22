FROM golang:1.12-alpine
WORKDIR /src/
ARG CI_JOB_TOKEN
RUN apk add --no-cache git mercurial make bash
RUN git clone -b vsphere --single-branch https://gitlab-ci-token:${CI_JOB_TOKEN}@fox.flant.com/sys/deckhouse-mcm.git /src/ && git checkout be97395974fc07f108a9dd0558c4bc18550af93a
RUN make build-local

FROM alpine:3.10

RUN apk add --no-cache bash curl

# Resources used by Golang libary
ENV ZONEINFO=/zone-info/zoneinfo.zip
COPY --from=0 /src/assets/zoneinfo.zip /zone-info/zoneinfo.zip

COPY --from=0 /src/bin/machine-controller-manager /machine-controller-manager
WORKDIR /
ENTRYPOINT ["/machine-controller-manager"]
