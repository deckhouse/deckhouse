ARG BASE_ALPINE
FROM $BASE_ALPINE

ADD start.sh /usr/local/bin/start.sh

RUN apk --update --no-cache add chrony tini iproute2 && \
    rm -rf /var/cache/apk/* /etc/chrony && \
    chmod 0755 /usr/local/bin/start.sh

WORKDIR /tmp
EXPOSE 123/udp
ENTRYPOINT ["tini", "--"]
CMD ["/usr/local/bin/start.sh"]
