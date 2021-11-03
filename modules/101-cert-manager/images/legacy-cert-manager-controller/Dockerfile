ARG BASE_ALPINE
FROM l.gcr.io/google/bazel:0.27.1@sha256:436708ebb76c0089b94c46adac5d3332adb8c98ef8f24cb32274400d01bde9e3 as artifact
RUN mkdir /build && cd /build \
  && git clone -b "v0.10.1" --single-branch https://github.com/jetstack/cert-manager.git
WORKDIR /build/cert-manager
RUN apt install -qy ca-certificates && update-ca-certificates 2>/dev/null
COPY patches/tolerations.patch ./
COPY patches/self_link.patch ./
ENV APP_VERSION v0.10.1
RUN patch -p1 < tolerations.patch && \
  patch -p1 < self_link.patch && \
  bazel build //cmd/controller --stamp=true

FROM $BASE_ALPINE as final
COPY --from=artifact /build/cert-manager/bazel-bin/cmd/controller/linux_amd64_pure_stripped/controller /bin/cert-manager-controller
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/bin/cert-manager-controller"]
