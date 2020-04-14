if bb-flag? is-bootstrapped; then exit 0; fi

yum install -y 'docker-ce-18.09*' 'docker-ce-cli-18.09*' containerd.io
yum versionlock "docker-ce-*"

mkdir -p /etc/docker
cat << "EOF" > /etc/docker/daemon.json
{
        "log-driver": "json-file",
        "log-opts": {
                "max-file": "5",
                "max-size": "10m"
        }
}
EOF

units="docker.service"

for unit in $units; do
  systemctl enable "$unit" && systemctl restart "$unit"
done
