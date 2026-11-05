# Static build of the toy aggregated API server. The build context is the
# repository root: the toy uses pkg/api-extension-sdk through a replace directive.
FROM golang:1.26 AS build
WORKDIR /src/modules/140-user-authz/docs/internal/scope-aggregated-api-e2e
COPY pkg/api-extension-sdk /src/pkg/api-extension-sdk
COPY modules/140-user-authz/docs/internal/scope-aggregated-api-e2e/go.mod modules/140-user-authz/docs/internal/scope-aggregated-api-e2e/go.sum ./
RUN go mod download
COPY modules/140-user-authz/docs/internal/scope-aggregated-api-e2e .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/toy-apiserver ./cmd/toy-apiserver

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/toy-apiserver /toy-apiserver
USER 65532:65532
ENTRYPOINT ["/toy-apiserver"]
