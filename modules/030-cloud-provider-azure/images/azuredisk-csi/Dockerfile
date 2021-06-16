# Based on https://github.com/kubernetes-sigs/azuredisk-csi-driver/blob/master/pkg/azurediskplugin/Dockerfile
ARG BASE_ALPINE
FROM mcr.microsoft.com/k8s/csi/azuredisk-csi:v0.8.0@sha256:d3d76a8904d0a7142aa88432d50100833a0834b64e2457beabf2b2ad84b65593 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /azurediskplugin /azurediskplugin
ENTRYPOINT [ "/azurediskplugin" ]
