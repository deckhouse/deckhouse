ARG BASE_PYTHON_ALPINE
FROM $BASE_PYTHON_ALPINE
RUN pip install ruamel.yaml
COPY control-plane-configurator.py /
ENTRYPOINT ["python3", "/control-plane-configurator.py"]
