# Based on https://github.com/falcosecurity/falcosidekick/blob/41d530807f1a0294c0276e4cb42af68c8b26a659/Dockerfile
ARG BASE_ALPINE
FROM $BASE_ALPINE
COPY --from=falcosecurity/falcosidekick:2.26.0-amd64@sha256:e13135c2cb2f01457dcf4ebca941c78898ad80c33c94a95e9b94033693411a31 /app /app
WORKDIR /app
ENTRYPOINT ["./falcosidekick"]
