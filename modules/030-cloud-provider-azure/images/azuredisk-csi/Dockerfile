# Based on https://github.com/kubernetes-sigs/azuredisk-csi-driver/blob/master/pkg/azurediskplugin/Dockerfile
ARG BASE_DEBIAN
FROM mcr.microsoft.com/k8s/csi/azuredisk-csi:v0.8.0@sha256:d3d76a8904d0a7142aa88432d50100833a0834b64e2457beabf2b2ad84b65593 as artifact

FROM $BASE_DEBIAN
RUN apt-get update && apt-get install -y util-linux e2fsprogs mount ca-certificates udev xfsprogs
COPY --from=artifact /azurediskplugin /azurediskplugin
ENTRYPOINT [ "/azurediskplugin" ]
