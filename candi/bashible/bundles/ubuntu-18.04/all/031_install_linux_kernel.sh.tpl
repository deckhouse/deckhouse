if bb-flag? is-bootstrapped; then exit 0; fi

export DEBIAN_FRONTEND=noninteractive

apt -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" install -qy linux-generic-hwe-18.04 linux-headers-generic-hwe-18.04 linux-image-generic-hwe-18.04

# TODO Ничего не делать если мы запущены уже на текущей версии
# TODO Прописать конкретную версию
# TODO Удалять все версии кроме текущей требуемой и той, с который мы сейчас запущены

{{- if ne .runType "ImageBuilding" }}
touch /var/lib/bashible/reboot
{{- end }}
