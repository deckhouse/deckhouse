SHELL := /bin/bash
cert_dir ?= "/tmp/d8-sds-drbd-operator-certs"
cert_cn ?= host.docker.internal

run: ## run go
	go run -race ./cmd/main.go

# make cert_cn=test.local create_certs
create_certs:
	mkdir -p $(cert_dir)
	openssl genrsa -out $(cert_dir)/root-ca-key.pem 2048
	openssl req -new -x509 -sha256 -key $(cert_dir)/root-ca-key.pem -out $(cert_dir)/root-ca.pem -days 3650 -subj '/CN=RootCA'
	openssl genrsa -out $(cert_dir)/tls.key 2048
	openssl req -new -key $(cert_dir)/tls.key -out $(cert_dir)/tls.csr -subj "/CN=$(cert_cn)" \
    -addext "subjectAltName = DNS:$(cert_cn), DNS:localhost"
	openssl x509 -req -in $(cert_dir)/tls.csr -CA $(cert_dir)/root-ca.pem -CAkey $(cert_dir)/root-ca-key.pem -CAcreateserial \
    -sha256 -out $(cert_dir)/tls.crt  -days 3650 \
    -extfile <(printf "subjectAltName=DNS:$(cert_cn), DNS:localhost")
	rm -f $(cert_dir)/tls.csr

run_with_created_certs:
	CERT_DIR=$(cert_dir) go run -race ./cmd/main.go

deploy:
