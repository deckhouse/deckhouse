ARG BASE_SHELL_OPERATOR
FROM $BASE_SHELL_OPERATOR
COPY hooks/ /hooks
RUN apk add --no-cache curl sqlite rsync python3 && \
  curl https://slugify.vercel.app/ > slugify && \
  chmod +x slugify && \
  mv slugify /usr/local/bin/
ENTRYPOINT ["python3", "/hooks/dashboard_provisioner.py"]
