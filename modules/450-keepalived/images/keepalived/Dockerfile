FROM python:3.7.4-alpine
RUN apk add --no-cache keepalived; pip3 install pyroute2; find /var/cache/apk/ -type f -delete
COPY prepare-config.py /
