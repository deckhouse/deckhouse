ARG BASE_SHELL_OPERATOR
ARG BASE_UBUNTU
FROM $BASE_SHELL_OPERATOR as shell-operator

FROM $BASE_UBUNTU
COPY --from=shell-operator /shell-operator /shell-operator
COPY requirements.txt requirements.txt
RUN \
    chmod +x /shell-operator \
    && apt update -y \
    && apt install -yq curl gnupg \
    && curl -s https://falco.org/repo/falcosecurity-packages.asc | apt-key add - \
    && echo "deb https://download.falco.org/packages/deb stable main" | tee -a /etc/apt/sources.list.d/falcosecurity.list \
    && apt update -y \
    && apt install -yq tini falco='0.35.0' python3='3.10.6-1~22.04' python3-pip \
    && pip3 install -r requirements.txt \
    # cleanup
    && apt-get clean autoclean \
    && apt-get autoremove -y \
    && rm -rf /var/lib/{apt,dpkg,cache,log}/

COPY hooks/ /hooks
ENV SHELL_OPERATOR_HOOKS_DIR /hooks
ENV LOG_TYPE json
ENV PYTHONPATH "/hooks"
ENTRYPOINT ["tini", "--", "/shell-operator"]
CMD ["start"]
