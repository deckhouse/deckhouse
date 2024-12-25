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
                <th>Image</th>
                <th>User</th>
                <th>Status</th>
            </tr>
        </thead>
        <tbody>
EOF

  # Генерируем строки таблицы из JSONL файла
  jq -r '. | @json' "$input_file" | while read -r line; do
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
  local image=$1
  local user
  local result
  local workdir=$2
  # Извлекаем информацию о пользователе из конфигурации образа
  user=$(crane config "$image" | jq '.config.User')
  if [ $user == "null" ] || [ "$user" == "root" ] || [ "$user" == "0:0" ]; then
    result="FAIL"
    echo "{\"Image\":\"$image\",\"User\":\"$user\",\"Status\":\"$result\"}" >> $workdir/report.jsonl
  fi
}

function base_images_tags() {
  base_images=$(grep . "$(pwd)/candi/image_versions.yml") # non empty lines
  base_images=$(grep -v "#" <<<"$base_images")            # remove comments

  reg_path=$(grep "REGISTRY_PATH" <<<"$base_images" | awk '{ print $2 }' | tr -d '"')

  base_images=$(grep -v "REGISTRY_PATH" <<<"$base_images") # Not an image
  base_images=$(grep -v "BASE_GOLANG" <<<"$base_images")   # golang images are used for multistage builds
  base_images=$(grep -v "BASE_JEKYLL" <<<"$base_images")   # images to build docs
  base_images=$(grep -v "BASE_NODE" <<<"$base_images")     # js bundles compilation

  base_images=$(awk '{ print $2 }' <<<"$base_images")                                          # pick an actual images address
  base_images=$(tr -d '"' <<<"$base_images") # "string" -> registry.deckhouse.io/base_images/string

  echo "$reg_path"
  echo "$base_images"
}

function __main__(){
  echo ""

  base_images_tags

  WORKDIR=$(mktemp -d)
  BASE_IMAGES_RAW=$(base_images_tags)
  REGISTRY=$(echo "$BASE_IMAGES_RAW" | head -n 1)
  BASE_IMAGES=$(echo "$BASE_IMAGES_RAW" | tail -n +2)
  mkdir -p out/
  htmlReportHeader > out/non-root-images.html

  # Проверка каждого образа
  for image in $BASE_IMAGES; do
    # Some of our base images contain no layers.
    # Trivy cannot scan such images because docker never implemented exporting them.
    # We should not attempt to scan images that cannot be exported.
    # Fixes https://github.com/deckhouse/deckhouse/issues/5020
    MANIFEST=$(echo ${REGISTRY}${image} | sed 's/:[^:@]*@/@/')
    docker manifest inspect $MANIFEST | jq -e '.layers | length > 0' > /dev/null || continue

    echo "----------------------------------------------"
    echo "👾 Image: $image"
    echo ""
    check_user $REGISTRY$image $WORKDIR
  done

  # Генерация HTML-отчёта
  generate_html_body $WORKDIR/report.jsonl >> out/non-root-images.html
  find "$WORKDIR" -type f -exec cat {} + | uniq | sort > out/.trivyignore
  rm -r "$WORKDIR"
  htmlReportFooter >> out/non-root-images.html
}

__main__
