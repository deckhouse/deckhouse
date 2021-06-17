ARG BASE_PYTHON_ALPINE
FROM $BASE_PYTHON_ALPINE
RUN apk add --no-cache keepalived; pip3 install pyroute2; find /var/cache/apk/ -type f -delete
COPY prepare-config.py /
