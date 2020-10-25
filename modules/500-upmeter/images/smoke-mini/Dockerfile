ARG BASE_GOLANG_ALPINE
ARG BASE_ALPINE
FROM $BASE_GOLANG_ALPINE
ADD go.mod /app/
WORKDIR /app
RUN go mod download
ADD smoke-mini.go /app/
RUN go build .

FROM $BASE_ALPINE
COPY --from=0 /app/smoke-mini /smoke-mini
