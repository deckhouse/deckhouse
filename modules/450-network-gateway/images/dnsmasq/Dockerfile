FROM python:3.7.9-alpine3.12

RUN apk add --no-cache dnsmasq; pip3 install pyroute2 six ipcalc; find /var/cache/apk/ -type f -delete

COPY prepare-config.py /
