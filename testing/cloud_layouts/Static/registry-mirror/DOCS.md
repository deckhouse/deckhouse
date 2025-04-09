commands to generate everything you need is here
```
BASTION_INTERNAL_IP=192.168.199.254
IMAGES_REPO=192.168.199.254:5001/sys/deckhouse-oss

./gen-ssl.sh

LOCAL_REGISTRY_MIRROR_PASSWORD=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 20; echo)
LOCAL_REGISTRY_CLUSTER_PASSWORD=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 20; echo)

./gen-auth-cfg.sh $LOCAL_REGISTRY_MIRROR_PASSWORD $LOCAL_REGISTRY_CLUSTER_PASSWORD > auth_config.yaml

LOCAL_REGISTRY_CLUSTER_DOCKERCFG=$(echo "cluster:${LOCAL_REGISTRY_CLUSTER_PASSWORD}" | base64)

env BASTION_INTERNAL_IP=$BASTION_INTERNAL_IP envsubst '$BASTION_INTERNAL_IP' < "registry-config.tpl.yaml" > "registry-config.yaml"
```

and docker compose up