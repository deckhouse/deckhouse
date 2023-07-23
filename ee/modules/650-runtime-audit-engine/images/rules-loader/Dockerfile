ARG BASE_SHELL_OPERATOR
ARG BASE_UBUNTU
FROM $BASE_SHELL_OPERATOR as shell-operator

FROM $BASE_UBUNTU
COPY --from=shell-operator /shell-operator /shell-operator
COPY requirements.txt requirements.txt
RUN \
    chmod +x /shell-operator \
    && apt update -y \
    && apt install -yq curl tini python3='3.10.6-1~22.04' python3-pip \
    && pip3 install -r requirements.txt \
    && curl -sSfL https://download.falco.org/packages/bin/x86_64/falco-0.35.1-x86_64.tar.gz | tar -C /tmp -xzvf - \
    && cp -f /tmp/falco-0.35.1-x86_64/usr/bin/falco /usr/bin/falco \
    # cleanup \
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
