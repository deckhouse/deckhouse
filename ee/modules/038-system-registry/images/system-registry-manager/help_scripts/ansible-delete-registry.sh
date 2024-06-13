#!/bin/bash

# Функция для нахождения и перехода в директорию, где находится скрипт
cd_script_dir() {
  # Определим путь к директории скрипта
  local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

  # Перейдем в эту директорию
  cd "$script_dir" || exit

  # Сообщим пользователю, в какую директорию мы перешли
  echo "Перешли в директорию скрипта: $script_dir"
}

run_ansible_playbook() {
  local inventory_file="$1"
  local playbook_file="$2"
  local ansible_options="$3"

  # Проверим, существует ли файл инвентаря
  if [ ! -f "$inventory_file" ]; then
    echo "Ошибка: Файл инвентаря '$inventory_file' не найден!"
    exit 1
  fi

  # Проверим, существует ли playbook
  if [ ! -f "$playbook_file" ]; then
    echo "Ошибка: Playbook '$playbook_file' не найден!"
    exit 1
  fi

  # Запуск Ansible playbook
  ansible-playbook -i "$inventory_file" $ansible_options "$playbook_file"

  # Проверка результата выполнения
  if [ $? -eq 0 ]; then
    echo "Playbook успешно выполнен!"
  else
    echo "Ошибка выполнения playbook!"
    exit 1
  fi
}

wait_for_pods_inactive() {
  local namespace="$1"
  local label_selector="$2"
  local sleep_interval="$3"

  while true; do
    if kubectl get pods -n "$namespace" -l="$label_selector" 2>&1 | grep -q "No resources found"; then
      echo "Все поды неактивны."
      break
    else
      echo "Поды все еще активны. Ждем..."
      sleep "$sleep_interval"
    fi
  done
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
################################################
#             Остановка manager-а              #
################################################
echo "Удаление system registry manager"
PATCH=$(cat <<EOF
[
  {
    "op": "replace",
    "path": "/spec/enabled",
    "value": false
  },
  {
    "op": "replace",
    "path": "/spec/settings/cluster/size",
    "value": 1
  }
]
EOF
)
kubectl_patch_module_config "ModuleConfig system-registry" "$PATCH"
wait_for_pods_inactive "d8-system" "app=system-registry-manager" 10

################################################
#             Остановка registry               #
################################################

echo "Удаление system registry"
run_ansible_playbook "inventory.yaml" "ansible-delete-registry.yaml" "--tags static-pods"
wait_for_pods_inactive "d8-system" "component=system-registry,tier=control-plane" 10
run_ansible_playbook "inventory.yaml" "ansible-delete-registry.yaml" "--tags data"

################################################
#           Удаление данных из etcd            #
################################################
echo "Очистка etcd"

etcd_pod_name=$(kubectl get pods -n kube-system -l=component=etcd,tier=control-plane -o jsonpath='{.items[0].metadata.name}')

if [ -z "$etcd_pod_name" ]; then
  echo "Ошибка: Под с etcd не найден!"
  exit 1
fi

# Получаем endpoint etcd с использованием jsonpath
etcd_endpoint=$(kubectl get pod "$etcd_pod_name" -n kube-system -o jsonpath='{.status.podIP}')

if [ -z "$etcd_endpoint" ]; then
  echo "Ошибка: Не удалось получить endpoint etcd!"
  exit 1
fi

kubectl exec -it pod/"$etcd_pod_name" -n kube-system etcd -- etcdctl \
--endpoints="$etcd_endpoint":2379 \
--cacert=/etc/kubernetes/pki/etcd/ca.crt \
--cert=/etc/kubernetes/pki/etcd/server.crt \
--key=/etc/kubernetes/pki/etcd/server.key \
del seaweedfs_meta. --prefix

# Проверка результата выполнения
if [ $? -eq 0 ]; then
  echo "Etcd успешно отчищен!"
else
  echo "Ошибка отчистки etcd!"
  exit 1
fi
