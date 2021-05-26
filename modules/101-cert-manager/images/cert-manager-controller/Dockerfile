ARG BASE_ALPINE
FROM l.gcr.io/google/bazel:0.27.1 as builder
RUN mkdir /build && cd /build \
  && git clone -b "v0.10.1" --single-branch https://github.com/jetstack/cert-manager.git
WORKDIR /build/cert-manager
RUN apt install -qy ca-certificates && update-ca-certificates 2>/dev/null
COPY tolerations.patch self_link.patch ./
ENV APP_VERSION v0.10.1
RUN patch -p1 < tolerations.patch && \
  patch -p1 < self_link.patch && \
  bazel build //cmd/controller --stamp=true

FROM $BASE_ALPINE as final
COPY --from=0 /build/cert-manager/bazel-bin/cmd/controller/linux_amd64_pure_stripped/controller /bin/cert-manager-controller
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/bin/cert-manager-controller"]
