ARG BASE_GOLANG_16_ALPINE
ARG BASE_ALPINE
FROM $BASE_GOLANG_16_ALPINE as artifact
ADD go.mod /app/
WORKDIR /app
RUN go mod download
ADD smoke-mini.go /app/
RUN go build .

FROM $BASE_ALPINE
COPY --from=artifact /app/smoke-mini /smoke-mini
