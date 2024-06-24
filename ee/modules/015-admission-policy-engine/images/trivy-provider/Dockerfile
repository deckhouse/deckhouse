ARG BASE_GOLANG_22_ALPINE
ARG BASE_DISTROLESS

FROM $BASE_GOLANG_20_ALPINE as artifact
WORKDIR /src/
COPY main.go go.mod go.sum /src/
COPY validators /src/validators/
COPY web /src/web/

RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o trivy-provider main.go


FROM $BASE_DISTROLESS
COPY --from=artifact /src/trivy-provider /bin/trivy-provider
ENTRYPOINT [ "/bin/trivy-provider" ]
