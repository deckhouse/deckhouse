ARG BASE_GOLANG_BUSTER
ARG BASE_DEBIAN
FROM $BASE_GOLANG_BUSTER
WORKDIR /src/
COPY 001-io1-io2-expand-support.patch 002-volume-stats.patch /
RUN apt-get update && apt-get install -y --no-install-recommends git mercurial patch
RUN wget https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.7.1.tar.gz -O - | tar -xz --strip-components=1 -C /src/ && \
  patch -p1 < /001-io1-io2-expand-support.patch && \
  patch -p1 < /002-volume-stats.patch
RUN make bin/aws-ebs-csi-driver

# support every standard Linux disk/mount utility so that CSI components won't complain
FROM $BASE_DEBIAN
RUN apt-get update && apt-get install -y ca-certificates e2fsprogs mount xfsprogs udev
COPY --from=0 /src/bin/aws-ebs-csi-driver /bin/aws-ebs-csi-driver
ENTRYPOINT [ "/bin/aws-ebs-csi-driver" ]
