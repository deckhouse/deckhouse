# Based on https://github.com/aws/aws-node-termination-handler/blob/main/Dockerfile
ARG BASE_ALPINE
FROM amazon/aws-node-termination-handler:v1.5.0-linux-amd64@sha256:4555874ffb9bd8c346507d41c788571ad462bbe9f9ee40bcda3d2a3329c54fb1 as artifact

FROM $BASE_ALPINE
COPY --from=artifact /node-termination-handler /node-termination-handler
ENTRYPOINT [ "/node-termination-handler" ]
