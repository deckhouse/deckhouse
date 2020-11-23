
ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE
FROM $BASE_GOLANG_ALPINE
WORKDIR /src/
RUN apk add --no-cache git mercurial
# TODO: switch to git tags once they become available in the upstream repository
RUN wget https://github.com/kubernetes/cloud-provider-gcp/archive/ca1e685df450fd5d32b57a659eb363b870b03937.tar.gz -O - | tar -xz --strip-components=1 -C /src/
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o gcp-cloud-controller-manager  cmd/cloud-controller-manager/controller-manager.go

FROM $BASE_ALPINE
COPY --from=0 /src/gcp-cloud-controller-manager /usr/local/bin/cloud-controller-manager
RUN apk add --no-cache curl ca-certificates && find /var/cache/apk/ -type f -delete
ENTRYPOINT [ "/usr/local/bin/cloud-controller-manager" ]
