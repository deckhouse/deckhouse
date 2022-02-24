ARG BASE_GOLANG_16_ALPINE
ARG BASE_ALPINE
FROM $BASE_GOLANG_16_ALPINE as artifact
ADD go.mod /app/
WORKDIR /app
RUN go mod download
ADD madison-proxy.go /app/
RUN go build .

FROM $BASE_ALPINE
COPY --from=artifact /app/madison-proxy /madison-proxy
CMD /madison-proxy
