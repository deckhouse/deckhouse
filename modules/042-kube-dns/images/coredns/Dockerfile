ARG BASE_GOLANG_17_ALPINE


FROM $BASE_GOLANG_17_ALPINE as artifact
WORKDIR /src
RUN apk add patch
RUN wget https://github.com/coredns/coredns/archive/refs/tags/v1.9.3.tar.gz -O - | tar -xzv --strip-components=1 -C /src/
COPY patches/support-alpha-tolerate-unready-endpoints-annotation.patch /src/
RUN patch -p1 < support-alpha-tolerate-unready-endpoints-annotation.patch
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/coredns/coredns/coremain.GitCommit=v1.9.3-flant.1" -o coredns

FROM cgr.dev/chainguard/apko:0.7.1 as apk_extractor

WORKDIR /workdir

COPY apko.yaml .

RUN apko build --arch x86_64 apko.yaml image:latest image.tar
RUN tar -xvf image.tar
RUN mkdir /rootfs && tar -xzvf *.tar.gz -C /rootfs

FROM scratch

COPY --from=apk_extractor /rootfs/ /

COPY --from=artifact /src/coredns /coredns
ENTRYPOINT [ "/coredns" ]
