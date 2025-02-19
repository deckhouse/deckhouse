while [[ "$#" -gt 0 ]]; do
  case $1 in
    --upload)
      upload=true
      upload_files=($2)
      shift
      ;;
    --message)
      message="$2"
      shift
      ;;
    *)
      echo "Unsupported argument: $1"
      exit 1
      ;;
  esac
  shift
done

if [[ -z "$message" ]]; then
  echo "Error: The --message flag is required and cannot be empty."
  exit 1
fi

token="${LOOP_TOKEN}"
channel_id="${LOOP_CHANNEL_ID}"
server_url="https://loop.flant.ru"
file_id_array=()

function upload_file() {
  file_id=$(curl -f -L -X POST "${server_url}/api/v4/files?channel_id=${channel_id}" \
  -H "Content-Type: multipart/form-data" \
  -H "Accept: application/json" \
  -H "Authorization: Bearer ${token}" \
  -F "data=@$1" | jq -M -c -r '.file_infos[].id' 2>/dev/null)

  echo "$file_id"
}

function send_post() {
  file_ids=$(IFS=,; echo "[${file_id_array[*]}]")

  curl -f -L -X POST "${server_url}/api/v4/posts" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${token}" \
    --data "{\"channel_id\": \"${channel_id}\",\"message\": \"${message}\",\"file_ids\": ${file_ids}}"
}

if [ "$upload" = true ]; then
  for file_path in ${upload_files[@]}; do
    file_id=$(upload_file "$file_path")
    file_id_array+=("$file_id")
  done
fi
send_post
