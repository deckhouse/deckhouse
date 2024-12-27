#!/usr/bin/env bash

set -e
source tools/cve/trivy-wrapper.sh
# Функция для генерации HTML тела
generate_html_body() {
  # Проверяем, что передан файл
  if [ -z "$1" ]; then
    echo "Usage: $0 <input.jsonl>"
    exit 1
  fi

  input_file="$1"

  # Начало тела HTML
  cat <<EOF
    <h1>Non-Root User Check Report</h1>
    <table>
        <thead>
            <tr>
                <th>ImageReportName</th>
                <th>Image</th>
                <th>User</th>
                <th>Status</th>
            </tr>
        </thead>
        <tbody>
EOF

  # Генерируем строки таблицы из JSONL файла
  jq -r '. | @json' "$input_file" | while read -r line; do
      imageReportName=$(echo "$line" | jq -r '.ImageReportName')
      image=$(echo "$line" | jq -r '.Image')
      user=$(echo "$line" | jq -r '.User')
      status=$(echo "$line" | jq -r '.Status')

      # Выбираем класс строки в зависимости от статуса
      if [ "$status" == "FAIL" ]; then
          row_class="fail"
      else
          row_class="pass"
      fi

      # Добавляем строку в таблицу
      cat <<ROW
          <tr class="$row_class">
              <td>$imageReportName</td>
              <td>$image</td>
              <td>$user</td>
              <td>$status</td>
          </tr>
ROW
  done

  # Закрытие тела HTML
  cat <<EOF
        </tbody>
    </table>
EOF
}
# Проверка запуска образа от пользователя root
function check_user() {
  local image=$2
  local user
  local result
  local image_report_name=$1
  local workdir=$3
  # Извлекаем информацию о пользователе из конфигурации образа
  user=$(crane config "$image" | jq '.config.User')
  if [ $user == "null" ] || [ "$user" == "root" ] || [ "$user" == "0:0" ]; then
    result="FAIL"
    if [ $user == "null" ];then
      user="root"
    fi 
    echo "{\"ImageReportName\":\"$image_report_name\",\"Image\":\"$image\",\"User\":\"$user\",\"Status\":\"$result\"}" >> $workdir/report.jsonl
  fi
}

function __main__() {
  echo "Deckhouse image to check non-root default user: $IMAGE:$TAG"
  echo "Severity: $SEVERITY"
  echo "----------------------------------------------"
  echo ""

  docker pull "$IMAGE:$TAG"
  digests=$(docker run --rm "$IMAGE:$TAG" cat /deckhouse/modules/images_digests.json)

  WORKDIR=$(mktemp -d)
  IMAGE_REPORT_NAME="deckhouse::$(echo "$IMAGE:$TAG" | sed 's/^.*\/\(.*\)/\1/')"
  mkdir -p out/
  htmlReportHeader > out/non-root-images.html
  check_user $IMAGE_REPORT_NAME $IMAGE:$TAG $WORKDIR
  for module in $(jq -rc 'to_entries[]' <<< "$digests"); do
    MODULE_NAME=$(jq -rc '.key' <<< "$module")
    echo "=============================================="
    echo "🛰 Module: $MODULE_NAME"

    for module_image in $(jq -rc '.value | to_entries[]' <<<"$module"); do
      IMAGE_NAME=$(jq -rc '.key' <<< "$module_image")
      if [[ "$IMAGE_NAME" == "trivy" ]]; then
        continue
      fi
      echo "----------------------------------------------"
      echo "👾 Image: $IMAGE_NAME"
      echo ""

      IMAGE_HASH="$(jq -rc '.value' <<< "$module_image")"
      IMAGE_REPORT_NAME="$MODULE_NAME::$IMAGE_NAME"
      check_user $IMAGE_REPORT_NAME "$IMAGE@$IMAGE_HASH" $WORKDIR
    done
  done
  # Генерация HTML-отчёта
  generate_html_body $WORKDIR/report.jsonl >> out/non-root-images.html
  rm -r "$WORKDIR"
  htmlReportFooter >> out/non-root-images.html
}

__main__
