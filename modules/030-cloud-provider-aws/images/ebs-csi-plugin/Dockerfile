ARG BASE_GOLANG_BUSTER
FROM $BASE_GOLANG_BUSTER
WORKDIR /src/
COPY volume-stats.patch /
RUN apt-get update && apt-get install -y --no-install-recommends git mercurial patch
RUN wget https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.4.0.tar.gz -O - | tar -xz --strip-components=1 -C /src/ && \
  patch -p1 < /volume-stats.patch
RUN make aws-ebs-csi-driver

# support every standard Linux disk/mount utility so that CSI components won't complain
FROM debian:buster
RUN apt-get update && apt-get install -y ca-certificates e2fsprogs mount xfsprogs udev
COPY --from=0 /src/bin/aws-ebs-csi-driver /bin/aws-ebs-csi-driver
ENTRYPOINT [ "/bin/aws-ebs-csi-driver" ]
