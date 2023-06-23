ARG BASE_ALPINE
ARG BASE_GOLANG_19_ALPINE

FROM $BASE_GOLANG_19_ALPINE AS build
WORKDIR /src
RUN apk add --update --no-cache patch
RUN wget https://github.com/aquasecurity/trivy/archive/refs/tags/v0.42.1.tar.gz -O - | tar -xz --strip-components=1

COPY patches/001-trivy-db-mediatype-hack.patch /src
COPY patches/002-insecure-registry-artifact.patch /src

RUN patch -p1 < 001-trivy-db-mediatype-hack.patch && \
    patch -p1 < 002-insecure-registry-artifact.patch

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w -extldflags "-static"' -o trivy ./cmd/trivy/main.go

FROM $BASE_ALPINE
COPY --from=build /src/trivy /usr/local/bin/
ENTRYPOINT [ "/usr/local/bin/trivy" ]
ENV LANG=C.UTF-8
