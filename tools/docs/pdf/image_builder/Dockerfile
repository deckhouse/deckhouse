FROM python:3.11.15-trixie

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    xz-utils \
    fonts-dejavu-core \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL -o /tmp/wkhtmltox.tar.xz \
    "https://github.com/wkhtmltopdf/wkhtmltopdf/releases/download/0.12.4/wkhtmltox-0.12.4_linux-generic-amd64.tar.xz" \
    && tar -xJf /tmp/wkhtmltox.tar.xz -C /opt \
    && rm /tmp/wkhtmltox.tar.xz

ENV PATH="/opt/wkhtmltox/bin:${PATH}"
# Prefer Qt/OpenSSL shipped with wkhtmltox over system OpenSSL 3 (QSslSocket / CRYPTO_*).
ENV LD_LIBRARY_PATH=/opt/wkhtmltox/lib

RUN pip install --no-cache-dir \
    beautifulsoup4 \
    pyyaml

WORKDIR /app
