# Based on https://github.com/kubernetes-sigs/azuredisk-csi-driver/blob/master/pkg/azurediskplugin/Dockerfile
ARG BASE_DEBIAN
FROM mcr.microsoft.com/k8s/csi/azuredisk-csi:v1.17.0@sha256:ae8763be94d8cf1802c06ad528be5f092ca0e97b611084f1f64adc925599d2b7 as artifact

FROM $BASE_DEBIAN
RUN apt-get update && apt-get install -y util-linux e2fsprogs mount ca-certificates udev xfsprogs
COPY --from=artifact /azurediskplugin /azurediskplugin
ENTRYPOINT [ "/azurediskplugin" ]
