ARG BASE_ALPINE
FROM stakater/reloader:v0.0.99@sha256:c2b2873c0b9aeaced969630eabc3a7d1f9bd92652687d853cf886db77bc482b7 AS reloader

FROM $BASE_ALPINE
COPY --from=reloader /manager /usr/local/bin/manager
ENTRYPOINT ["/usr/local/bin/manager"]
