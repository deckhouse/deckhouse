ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE

FROM $BASE_GOLANG_20_ALPINE AS build
ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}
WORKDIR /src
RUN apk add --update --no-cache patch git
RUN git clone --depth 1 --branch v0.14.0 ${SOURCE_REPO}/aquasecurity/trivy-operator.git .

COPY patches/001-add-registry-secret-as-dockerconfigjson.patch /src
COPY patches/002-skip-some-checks.patch /src

RUN patch -p1 < 001-add-registry-secret-as-dockerconfigjson.patch && \
    patch -p1 < 002-skip-some-checks.patch

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w -extldflags "-static"' -o operator-trivy ./cmd/trivy-operator/main.go

RUN chown 64535:64535 operator-trivy
RUN chmod 0700 operator-trivy

FROM $BASE_DISTROLESS
COPY --from=build /src/operator-trivy /
ENTRYPOINT [ "/operator-trivy" ]
