# Based on https://github.com/falcosecurity/falcosidekick/blob/41d530807f1a0294c0276e4cb42af68c8b26a659/Dockerfile
ARG BASE_GOLANG_18_ALPINE
ARG BASE_DISTROLESS

FROM $BASE_GOLANG_18_ALPINE as artifact
ARG GOPROXY
ENV GOPROXY=${GOPROXY}

ARG SOURCE_REPO
ENV SOURCE_REPO=${SOURCE_REPO}

WORKDIR /src
RUN apk add --no-cache make git bash && \
    git clone --branch 2.26.0 --depth 1 ${SOURCE_REPO}/falcosecurity/falcosidekick.git . && \
    make falcodisekick && \
    chown -R 64535:64535 /src/falcosidekick && \
    chmod 0700 /src/falcosidekick

FROM $BASE_DISTROLESS
COPY --from=artifact /src/falcosidekick /falcosidekick
ENTRYPOINT [ "/falcosidekick" ]

