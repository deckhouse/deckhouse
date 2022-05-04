ARG BASE_ALPINE
FROM paskalmaksim/aks-node-termination-handler:v1.0.0-linux-amd64@sha256:ed28eae7a59b18ec84588fda55a660e371dfd71c9ad86e36cffa271d703ab97b as artifact

FROM $BASE_ALPINE
COPY --from=artifact /app/aks-node-termination-handler /node-termination-handler
ENTRYPOINT [ "/node-termination-handler" ]
