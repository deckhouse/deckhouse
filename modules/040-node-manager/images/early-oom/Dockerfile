ARG BASE_ALPINE
FROM $BASE_ALPINE as artifact
WORKDIR /src
COPY "psi-monitor.c" /src/

RUN apk add --no-cache gcc musl-dev
RUN gcc -o psi-monitor psi-monitor.c

FROM $BASE_ALPINE
COPY --from=artifact /src/psi-monitor /
ENTRYPOINT [ "/psi-monitor" ]
