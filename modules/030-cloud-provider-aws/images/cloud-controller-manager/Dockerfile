FROM golang:1.13-alpine
WORKDIR /src/
RUN apk add --no-cache git mercurial make bash
RUN wget https://github.com/kubernetes/cloud-provider-aws/archive/8a0b2ca4b44f348c977c6d6d1da7f59ea07e3be6.tar.gz -O - | tar -xz --strip-components=1 -C /src
RUN make aws-cloud-controller-manager

FROM alpine:3.10

COPY --from=0 /src/aws-cloud-controller-manager /usr/local/bin/aws-cloud-controller-manager
WORKDIR /
ENTRYPOINT ["/usr/local/bin/aws-cloud-controller-manager"]
