#!/bin/bash
set -e

main() {
  NAMESPACE='antiopa'
  DRY_RUN=0

  parse_args "$@" || (usage && exit 1)

  delete_antiopa

  return $?
}


usage() {
printf " Usage: $0 [--dry-run]

    --dry-run
            Do not run kubectl apply.
            Print yaml to stdout or to -o file.

    --help|-h
            Print this message.

"
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --dry-run)
        DRY_RUN=1
        ;;
      --help|-h)
        return 1
        ;;
      --*)
        echo "Illegal option $1"
        return 1
        ;;
    esac
    shift $(( $# > 0 ? 1 : 0 ))
  done
}

delete_antiopa() {
  if [[ $DRY_RUN == 1 ]]; then
    ECHO="echo "
  fi
  $ECHO kubectl delete clusterrolebinding ${NAMESPACE}
  $ECHO kubectl delete namespace ${NAMESPACE}
}


# wait for full file download if executed as
# $ curl | sh
main "$@"
