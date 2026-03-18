# Docker Registry Ansible Playbook

Ansible playbook for deploying a Docker Registry (distribution v3) with token-based authentication (cesanta/docker_auth), Nginx reverse proxy (mainline from nginx.org), and Let's Encrypt TLS on Ubuntu 24.04.

## Architecture

```
Client --HTTPS:443--> Nginx --/v2/--> Registry (127.0.0.1:5000)
                            \--/auth--> docker_auth (127.0.0.1:5001)
```

- **Registry** and **docker_auth** run as systemd services (bare binaries extracted from container images via [crane](https://github.com/google/go-containerregistry/tree/main/cmd/crane), no Docker daemon required).
- **Nginx** from nginx.org mainline terminates TLS and proxies requests.
- **Certbot** obtains and auto-renews Let's Encrypt certificates.
- **Garbage collection** runs daily via a systemd timer.

## Prerequisites

- Ansible >= 2.14 on the controller
- `python3-passlib` and `python3-bcrypt` on the controller (installed automatically by the playbook)
- Target machine running Ubuntu 24.04 with DNS A record pointing `dev-registry.okmetric.com` to the server IP
- Port 80 and 443 open on the target

## Quick Start

1. Edit the inventory:

```bash
vim inventory/hosts.yml
```

2. Set passwords (use ansible-vault for production):

```bash
# Option A: edit group_vars directly
vim group_vars/all.yml

# Option B: use extra vars
ansible-playbook -i inventory/hosts.yml playbook.yml \
  -e registry_admin_password='s3cur3-admin-pw' \
  -e registry_reader_password='s3cur3-reader-pw' \
  -e certbot_email='you@example.com'

# Option C: encrypt with ansible-vault
ansible-vault encrypt_string 's3cur3-admin-pw' --name registry_admin_password
```

3. Run the playbook:

```bash
ansible-playbook -i inventory/hosts.yml playbook.yml
```

## Usage

After deployment:

```bash
# Log in
docker login dev-registry.okmetric.com

# Push an image
docker tag myimage:latest dev-registry.okmetric.com/myimage:latest
docker push dev-registry.okmetric.com/myimage:latest

# Pull an image
docker pull dev-registry.okmetric.com/myimage:latest
```

## Users and ACL

| User      | Permissions              |
|-----------|--------------------------|
| `admin`   | superuser (`*`)          |
| `rw`      | pull, push               |
| `ro`      | pull only                |
| `cleanup` | pull, push, delete       |

Users and ACL are defined in `registry_users` and `registry_acl` variables. Passwords default to `changeme` -- override via `--extra-vars` or ansible-vault.

## Key Variables

| Variable                 | Default                        | Description                            |
|--------------------------|--------------------------------|----------------------------------------|
| `registry_domain`        | `dev-registry.okmetric.com`    | Registry FQDN                          |
| `registry_version`       | `3.0.0`                        | Distribution version                   |
| `docker_auth_version`    | `1.14.0`                       | docker_auth version                    |
| `registry_data_dir`      | `/var/lib/registry`            | Registry storage directory             |
| `registry_users`         | 4 users (see defaults)         | Dict of users with passwords           |
| `registry_acl`           | see defaults                   | ACL policy list                        |
| `certbot_email`          | `admin@okmetric.com`           | Email for Let's Encrypt                |
| `gc_calendar`            | `*-*-* 03:00:00`              | Garbage collection schedule            |

## Systemd Services

| Service              | Description                              |
|----------------------|------------------------------------------|
| `docker-registry`    | Docker Registry (distribution)           |
| `docker-auth`        | Token auth server (cesanta/docker_auth)  |
| `nginx`              | Reverse proxy with TLS                   |
| `registry-gc.timer`  | Daily garbage collection timer           |

```bash
# Check status
systemctl status docker-registry docker-auth nginx registry-gc.timer

# View logs
journalctl -u docker-registry -f
journalctl -u docker-auth -f

# Manual garbage collection
systemctl start registry-gc.service
```

## File Layout on Target

```
/usr/local/bin/crane              # crane (image export tool)
/usr/local/bin/registry           # Registry binary
/usr/local/bin/docker_auth        # docker_auth binary
/usr/local/bin/registry-gc.sh     # GC script
/etc/docker-registry/config.yml   # Registry config
/etc/docker-registry/token.crt    # JWT verification certificate
/etc/docker-auth/config.yml       # docker_auth config
/etc/docker-auth/token.key        # JWT signing key
/etc/docker-auth/token.crt        # JWT certificate
/etc/nginx/conf.d/registry.conf   # Nginx vhost
/var/lib/registry/                # Image storage
```
