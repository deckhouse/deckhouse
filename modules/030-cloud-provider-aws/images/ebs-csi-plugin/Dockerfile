ARG BASE_GOLANG_BUSTER
ARG BASE_DEBIAN
FROM $BASE_GOLANG_BUSTER as artifact
WORKDIR /src/
RUN wget https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v1.3.0.tar.gz -O - | tar -xz --strip-components=1 -C /src/
RUN make bin/aws-ebs-csi-driver

# support every standard Linux disk/mount utility so that CSI components won't complain
FROM $BASE_DEBIAN
RUN apt-get update && apt-get install -y ca-certificates e2fsprogs mount xfsprogs udev
COPY --from=artifact /src/bin/aws-ebs-csi-driver /bin/aws-ebs-csi-driver
ENTRYPOINT [ "/bin/aws-ebs-csi-driver" ]
