function tools::generate_password() {
  pwgen -s 20 1
}

function tools::is_empty() {
  [[ -z "${1:-}" || "${1:-}" == "null" ]]
}
