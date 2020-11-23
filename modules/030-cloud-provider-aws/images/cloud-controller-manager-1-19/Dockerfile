# once upstream cloud-provider-aws migrates from legacy-cloud-providers, we should change the build process

ARG BASE_ALPINE
ARG BASE_GOLANG_ALPINE

FROM $BASE_GOLANG_ALPINE as legacy-cloud-providers
WORKDIR /src/
RUN apk add --no-cache git mercurial patch
RUN wget https://github.com/kubernetes/legacy-cloud-providers/archive/v0.19.3.tar.gz -O - | tar -xz --strip-components=1 -C /src
COPY legacy-cloud-providers-patches/001-identify-instances-by-name.patch \
    legacy-cloud-providers-patches/002-non-type-lb.patch /src/
RUN patch -p1 < 001-identify-instances-by-name.patch && \
    patch -p1 < 002-non-type-lb.patch

FROM $BASE_GOLANG_ALPINE as ccm
COPY --from=legacy-cloud-providers /src /legacy-cloud-providers
WORKDIR /src/
RUN apk add --no-cache git mercurial make bash patch
# TODO: switch to git tags once they become available in the upstream repository
RUN wget https://github.com/kubernetes/cloud-provider-aws/archive/b390ec15ea471adc8312b4528cf37814852b993e.tar.gz -O - | tar -xz --strip-components=1 -C /src
COPY 001-simplify-go-mod.patch /src
RUN patch -p1 < 001-simplify-go-mod.patch
# pay attention when changing upstream version, k8s.io/legacy-cloud-providers may change, which will lead to unexpected results
RUN go mod edit -replace=k8s.io/legacy-cloud-providers=/legacy-cloud-providers
RUN make aws-cloud-controller-manager

FROM $BASE_ALPINE
COPY --from=ccm /src/aws-cloud-controller-manager /usr/local/bin/aws-cloud-controller-manager
WORKDIR /
ENTRYPOINT ["/usr/local/bin/aws-cloud-controller-manager"]
