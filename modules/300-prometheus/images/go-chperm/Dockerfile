ARG BASE_DISTROLESS
ARG BASE_GOLANG_20_ALPINE

FROM $BASE_GOLANG_20_ALPINE as chperm
COPY /src/ /src/
WORKDIR /src/
ARG GOPROXY
ENV GOPROXY=${GOPROXY} \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64
RUN go build -ldflags="-s -w" -o chperm

FROM $BASE_DISTROLESS
COPY --from=chperm /src/chperm /usr/local/bin/
ENTRYPOINT ["chperm"]
