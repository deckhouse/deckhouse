FROM python:3.7.9-alpine3.12

RUN apk --no-cache add iptables; find /var/cache/apk/ -type f -delete

COPY iptables-loop.py /
