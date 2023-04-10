ARG BASE_ALPINE
ARG BASE_GOLANG_19_ALPINE

FROM $BASE_GOLANG_19_ALPINE AS build
WORKDIR /src
RUN apk add --update --no-cache patch
RUN wget https://github.com/aquasecurity/trivy-operator/archive/refs/tags/v0.13.0.tar.gz -O - | tar -xz --strip-components=1 && \
    go mod vendor

COPY patches/001-add-registry-secret-as-dockerconfigjson.patch /src
COPY patches/002-skip-some-checks.patch /src
COPY patches/003-fix-defsec-lib-for-cis-compability.patch /src
COPY patches/004-fix-defsec-lib-for-deckhouse-setup.patch /src

RUN patch -p1 < 001-add-registry-secret-as-dockerconfigjson.patch && \
    patch -p1 < 002-skip-some-checks.patch && \
    patch -p1 < 003-fix-defsec-lib-for-cis-compability.patch && \
    patch -p1 < 004-fix-defsec-lib-for-deckhouse-setup.patch

RUN CGO_ENABLED=0 GOOS=linux go build -mod vendor -ldflags '-s -w -extldflags "-static"' -o operator-trivy ./cmd/trivy-operator/main.go

FROM $BASE_ALPINE
COPY --from=build /src/operator-trivy /
ENTRYPOINT [ "/operator-trivy" ]
ENV LANG=C.UTF-8
