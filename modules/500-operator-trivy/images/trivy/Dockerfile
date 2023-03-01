ARG BASE_ALPINE
ARG BASE_GOLANG_19_ALPINE

FROM $BASE_GOLANG_19_ALPINE AS build
WORKDIR /src
RUN wget https://github.com/aquasecurity/trivy/archive/refs/tags/v0.37.3.tar.gz -O - | tar -xz --strip-components=1
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w -extldflags "-static"' -o trivy ./cmd/trivy/main.go

FROM $BASE_ALPINE
COPY --from=build /src/trivy /usr/local/bin/
ENTRYPOINT [ "/usr/local/bin/trivy" ]
ENV LANG=C.UTF-8
