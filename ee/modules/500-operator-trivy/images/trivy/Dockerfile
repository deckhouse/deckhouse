ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE
ARG SOURCE_REPO

FROM $BASE_GOLANG_20_ALPINE AS build
ENV SOURCE_REPO=${SOURCE_REPO}
WORKDIR /src
RUN apk add git
RUN git clone --branch v0.42.0 --depth 1 ${SOURCE_REPO}/aquasecurity/trivy.git .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w -extldflags "-static"' -o trivy ./cmd/trivy/main.go

RUN chown 64535:64535 trivy
RUN chmod 0700 trivy

FROM $BASE_DISTROLESS
COPY --from=build /src/trivy /usr/local/bin/
ENTRYPOINT [ "/usr/local/bin/trivy" ]
