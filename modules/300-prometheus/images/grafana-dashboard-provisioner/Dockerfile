ARG BASE_PYTHON_ALPINE
FROM $BASE_PYTHON_ALPINE
COPY requirements.txt hooks/dashboard_provisioner.py /
WORKDIR /
RUN pip3 install --upgrade pip && pip3 install -r requirements.txt && \apk add --no-cache curl sqlite rsync python3 jq
ENTRYPOINT ["python3", "/dashboard_provisioner.py"]
