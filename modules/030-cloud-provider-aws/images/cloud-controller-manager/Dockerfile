FROM golang:1.13-alpine
WORKDIR /src/
RUN apk add --no-cache git mercurial make bash
RUN wget https://github.com/flant/cloud-provider-aws/archive/v0.17.1-flant.7.tar.gz -O - | tar -xz --strip-components=1 -C /src
RUN make aws-cloud-controller-manager

FROM alpine:3.10

COPY --from=0 /src/aws-cloud-controller-manager /usr/local/bin/aws-cloud-controller-manager
WORKDIR /
ENTRYPOINT ["/usr/local/bin/aws-cloud-controller-manager"]
