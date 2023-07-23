ARG BASE_SHELL_OPERATOR
ARG BASE_UBUNTU
FROM $BASE_SHELL_OPERATOR as shell-operator

FROM $BASE_UBUNTU as falco-builder
COPY requirements.txt requirements.txt
RUN \
    apt update -y \
    && apt install -yq curl python3='3.10.6-1~22.04' python3-pip \
    && pip3 install -r requirements.txt \
    && curl -sSfL https://download.falco.org/packages/bin/x86_64/falco-0.35.1-x86_64.tar.gz | tar -C /tmp -xzvf - \
    && cp -f /tmp/falco-0.35.1-x86_64/usr/bin/falco /usr/bin/falco \
    && cp -rf /tmp/falco-0.35.1-x86_64/usr/share/falco /usr/share/falco \
    # cleanup
    && apt-get clean autoclean \
    && apt-get autoremove -y \
    && rm -rf /var/lib/{apt,dpkg,cache,log}/ \
    && rm -rf /tmp/falco-*

FROM $BASE_UBUNTU
COPY --from=shell-operator /shell-operator /shell-operator
COPY --from=falco-builder /usr/bin/falco /usr/bin/falco
COPY --from=falco-builder /usr/share/falco /usr/share/falco
COPY --from=falco-builder /usr/local/lib/python3.10 /usr/local/lib/python3.10
RUN \
    chmod +x /shell-operator \
    && apt update -y \
    && apt install -yq curl tini python3='3.10.6-1~22.04' \
    # cleanup
    && apt-get clean autoclean \
    && apt-get autoremove -y \
    && rm -rf /var/lib/{apt,dpkg,cache,log}/ \
    && rm -rf /tmp/falco-*

COPY hooks/ /hooks
ENV SHELL_OPERATOR_HOOKS_DIR /hooks
ENV LOG_TYPE json
ENV PYTHONPATH "/hooks"
ENTRYPOINT ["tini", "--", "/shell-operator"]
CMD ["start"]
