ARG BASE_GOLANG_19_ALPINE
ARG BASE_ALPINE
FROM $BASE_GOLANG_19_ALPINE as artifact
ADD go.mod /app/
WORKDIR /app
RUN go mod download
ADD alerts-receiver.go /app/
RUN go build .

FROM $BASE_ALPINE
COPY --from=artifact /app/alerts-receiver /alerts-receiver
CMD /alerts-receiver
