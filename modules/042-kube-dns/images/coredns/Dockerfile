# Based on https://github.com/coredns/coredns/blob/master/Dockerfile
ARG BASE_ALPINE
ARG BASE_GOLANG_17_ALPINE


FROM $BASE_GOLANG_17_ALPINE as artifact
WORKDIR /src
RUN apk add patch
RUN wget https://github.com/coredns/coredns/archive/refs/tags/v1.9.3.tar.gz -O - | tar -xzv --strip-components=1 -C /src/
COPY patches/support-alpha-tolerate-unready-endpoints-annotation.patch /src/
RUN patch -p1 < support-alpha-tolerate-unready-endpoints-annotation.patch
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/coredns/coredns/coremain.GitCommit=v1.9.3-flant.1" -o coredns


FROM $BASE_GOLANG_17_ALPINE
RUN apk add --no-cache ca-certificates
COPY --from=artifact /src/coredns /coredns
ENTRYPOINT [ "/coredns" ]
