FROM python:3.7.4-alpine

RUN apk --no-cache add iptables; find /var/cache/apk/ -type f -delete

COPY iptables-loop.py /
