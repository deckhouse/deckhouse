ARG BASE_PYTHON_ALPINE
FROM $BASE_PYTHON_ALPINE

RUN apk --no-cache add iptables; find /var/cache/apk/ -type f -delete

COPY iptables-loop.py /
