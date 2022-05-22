ARG BASE_ALPINE
ARG BASE_GOLANG_17_BUSTER
FROM $BASE_GOLANG_17_BUSTER as artifact
RUN mkdir /build && cd /build \
  && git clone -b "v1.7.1" --single-branch https://github.com/jetstack/cert-manager.git
WORKDIR /build/cert-manager
RUN apt-get update && apt-get install -qy ca-certificates patch && update-ca-certificates 2>/dev/null
RUN curl -sSfL https://github.com/bazelbuild/bazelisk/releases/download/v1.11.0/bazelisk-linux-amd64 -o /usr/local/bin/bazel && chmod +x /usr/local/bin/bazel
COPY patches/certificate_owner_ref.patch ./
ENV APP_VERSION v1.7.1
ENV USE_BAZEL_VERSION 4.2.1
RUN patch -p1 < certificate_owner_ref.patch && \
  bazel build //cmd/controller --stamp=true

FROM $BASE_ALPINE as final
COPY --from=artifact /build/cert-manager/bazel-bin/cmd/controller/linux_amd64_pure_stripped/controller /bin/cert-manager-controller
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/bin/cert-manager-controller"]
