ARG BASE_GOLANG_22_BULLSEYE
ARG BASE_DISTROLESS
ARG TRIVY_VERSION=v0.55.0-flant

FROM $BASE_GOLANG_22_BULLSEYE as artifact
WORKDIR /src/
COPY main.go go.mod go.sum /src/
COPY validators /src/validators/
COPY web /src/web/

RUN git clone --depth 1 --branch ${TRIVY_VERSION} ${SOURCE_REPO}/aquasecurity/trivy.git 
RUN go mod edit -replace github.com/aquasecurity/trivy=./trivy
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o trivy-provider main.go


FROM $BASE_DISTROLESS
COPY --from=artifact /src/trivy-provider /bin/trivy-provider
ENTRYPOINT [ "/bin/trivy-provider" ]
