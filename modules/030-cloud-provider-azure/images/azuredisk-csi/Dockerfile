# Based on https://github.com/kubernetes-sigs/azuredisk-csi-driver/blob/master/pkg/azurediskplugin/Dockerfile
ARG BASE_DEBIAN
FROM mcr.microsoft.com/k8s/csi/azuredisk-csi:v1.17.0@sha256:bef83c3ad0b0d4e4e970aeaa8fe708c704cc50ca19d112262d6326f680904d9e as artifact

FROM $BASE_DEBIAN
RUN apt-get update && apt-get install -y util-linux e2fsprogs mount ca-certificates udev xfsprogs
COPY --from=artifact /azurediskplugin /azurediskplugin
ENTRYPOINT [ "/azurediskplugin" ]
