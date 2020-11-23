ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE
WORKDIR /src/
RUN apk add --no-cache git mercurial make bash
RUN wget https://github.com/flant/cloud-provider-aws/archive/635c59939c660695e1d517ad92b260ae6870223f.tar.gz -O - | tar -xz --strip-components=1 -C /src
RUN make aws-cloud-controller-manager

FROM $BASE_ALPINE

COPY --from=0 /src/aws-cloud-controller-manager /usr/local/bin/aws-cloud-controller-manager
WORKDIR /
ENTRYPOINT ["/usr/local/bin/aws-cloud-controller-manager"]
