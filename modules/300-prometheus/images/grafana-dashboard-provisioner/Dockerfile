ARG BASE_PYTHON_ALPINE
FROM $BASE_PYTHON_ALPINE
COPY hooks/dashboard_provisioner.py /
WORKDIR /
RUN apk add --no-cache curl sqlite rsync python3 jq
ENTRYPOINT ["python3", "/dashboard_provisioner.py"]
