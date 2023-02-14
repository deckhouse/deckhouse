ARG BASE_ALPINE
ARG BASE_GOLANG_19_ALPINE

FROM $BASE_GOLANG_19_ALPINE AS build
WORKDIR /src
RUN wget https://github.com/deckhouse/k8s-image-availability-exporter/tarball/v0.3.4 -O - | tar -xz --strip-components=1
RUN CGO_ENABLED=0 GOOS=linux go build -o trivy-operator ./cmd/trivy-operator/main.go

FROM $BASE_ALPINE
COPY --from=build /src/operator-trivy /
ENTRYPOINT [ "/operator-trivy" ]
ENV LANG=C.UTF-8
