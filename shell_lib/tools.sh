function tools::generate_password() {
  pwgen -s 20 1
}

function tools::to_slug() {
  to_slug() {
    # Forcing the POSIX local so alnum is only 0-9A-Za-z
    export LANG=POSIX
    export LC_ALL=POSIX
    # Keep only alphanumeric value
    sed -e 's/[^[:alnum:]]/-/g' |
    # Keep only one dash if there is multiple one consecutively
    tr -s '-'                   |
    # Lowercase everything
    tr A-Z a-z                  |
    # Remove last dash if there is nothing after
    sed -e 's/-$//'
  }

  # Consume stdin if it exist
  if test -p /dev/stdin; then
    read -r input
  fi

  # Now check if there was input in stdin
  if test -n "${input}"; then
    echo "${input}" | to_slug
    exit
  # No stdin, let's check if there is an argument
  elif test -n "${1}"; then
    echo "${1}" | to_slug
    exit
  else
    >&2 echo "ERROR: no input found to slugify"
    return 1
  fi
}

function tools::dk_convert {
  local config=""
  local val=""
  if [[ "$1" == "--milli" ]] ; then
    config=$1
    shift
  fi
  val=$1
  case $config in
    "--milli")
      if ! deckhouse-controller helper unit convert --mode kube-resource-unit --output milli <<< "$val" 2>/dev/null; then
        >&2 echo "ERROR: input value $1 must be in Quantity format !"
        return 1
      fi
      ;;
    *)
      if ! deckhouse-controller helper unit convert --mode kube-resource-unit <<< "$val" 2>/dev/null; then
        >&2 echo "ERROR: input value $1 must be in Quantity format !"
        return 1
      fi
      ;;
  esac
}
