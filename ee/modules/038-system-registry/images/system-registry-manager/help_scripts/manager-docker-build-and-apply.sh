#! /bin/bash

set -e

# Входные параметры:
DECKHOUSE_PATH="/Users/vadimmartynov/Data/Flant/deckhouse/"
IMG_PREFIX="vadim"
DOCKER_REGISTRY="cr.yandex/crp8n201pre28pm81udl/sys/deckhouse-oss"
PATCH_MODULE_CONFIG_BY_IMAGE_AMD64_NAME_DIGEST=true



IMG_NAME="$DOCKER_REGISTRY/$IMG_PREFIX-manager"
IMG_NAME_LATEST="$IMG_NAME:latest"


# Функция для нахождения и перехода в директорию, где находится скрипт
cd_script_dir() {
  # Определим путь к директории скрипта
  local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

  # Перейдем в эту директорию
  cd "$script_dir" || exit

  # Сообщим пользователю, в какую директорию мы перешли
  echo "Перешли в директорию скрипта: $script_dir"
}

# Функция для применения патча
kubectl_patch_module_config() {
  local args="$1"
  local patch="$2"

  # Применение патча
  kubectl patch $args --type='json' -p "$patch"

  # Проверка результата выполнения
  if [ $? -eq 0 ]; then
    echo "Патч успешно применен!"
  else
    echo "Ошибка применения патча!"
    exit 1
  fi
}

cd_script_dir

if ! docker buildx inspect mybuilder > /dev/null 2>&1; then
    # Если не существует, создаем его
    docker buildx create --name mybuilder --driver docker-container --use
else
    # Если существует, просто переключаемся на него
    docker buildx use mybuilder
fi
docker buildx build $DECKHOUSE_PATH \
    -f Manager.Dockerfile \
    --platform linux/amd64,linux/arm64 \
    -t $IMG_NAME_LATEST \
    --push

echo "------------------------------------------------------------------------------------------"
echo "Образ успешно собран и запушен, полное имя образа: '$IMG_NAME_LATEST'"
echo "------------------------------------------------------------------------------------------"

IMAGE_AMD64_DIGEST=$(docker buildx imagetools inspect $IMG_NAME_LATEST --raw | jq -r '.manifests[] | select(.platform.architecture == "amd64").digest')
IMAGE_AMD64_NAME_DIGEST="$IMG_NAME@$IMAGE_AMD64_DIGEST"
echo "------------------------------------------------------------------------------------------"
echo "Дайджест образа: '$IMAGE_AMD64_NAME_DIGEST'"
echo "------------------------------------------------------------------------------------------"

if [ "$PATCH_MODULE_CONFIG_BY_IMAGE_AMD64_NAME_DIGEST" = "true" ]; then
    ################################################
    #       Обновление образа в ConfigModule       #
    ################################################
    echo "Обновление образа registry manager"
    PATCH=$(cat <<EOF
[
  {
    "op": "replace",
    "path": "/spec/settings/imagesOverride/registryManager",
    "value": "$IMAGE_AMD64_NAME_DIGEST"
  }
]
EOF
)
    kubectl_patch_module_config "ModuleConfig system-registry" "$PATCH"
fi
