ARG BASE_ALPINE
FROM quay.io/jetstack/cert-manager-controller:v1.7.1@sha256:51027a4cc4d30e197e3506daf3a4fa2d2a0bc2826469f8a87848dfd279e031c0 as artifact
FROM $BASE_ALPINE as final
COPY --from=artifact /app/cmd/controller/controller /bin/cert-manager-controller
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/bin/cert-manager-controller"]


ARG BASE_ALPINE
FROM l.gcr.io/google/bazel:3.5.0@sha256:ace9881e6e9c5d48b5fd637321361aeffe54000265894a65f7d818dc1065bd80 as artifact
RUN mkdir /build && cd /build \
  && git clone -b "v1.17.1" --single-branch https://github.com/jetstack/cert-manager.git
WORKDIR /build/cert-manager
RUN apt install -qy ca-certificates && update-ca-certificates 2>/dev/null
COPY patches/certificate_owner_ref.patch ./
ENV APP_VERSION v1.17.1
RUN patch -p1 < certificate_owner_ref.patch && \
  bazel build //cmd/controller --stamp=true

FROM $BASE_ALPINE as final
COPY --from=artifact /build/cert-manager/bazel-bin/cmd/controller/linux_amd64_pure_stripped/controller /bin/cert-manager-controller
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/bin/cert-manager-controller"]
