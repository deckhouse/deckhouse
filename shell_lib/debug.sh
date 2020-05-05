#!/bin/bash

function debug::breakpoint() {
  ip="$1"
  port="$2"

  if netstat -nlt | grep -q "$ip:$port.*LISTEN"; then
    echo "ERROR: Failed to listen on $ip:$port: Already in use"
    return 1
  fi

  coproc nc -l -C $ip $port

  echo "#############|  Start of DEBUG session  |####################" >&"${COPROC[1]}"
  cat >&"${COPROC[1]}" <<END
    * !continue to end DEBUG session (hook will continue)
    * any other comand will be evalueted in breakpoint context
#############################################################

END

  while kill -0 "$COPROC_PID"; do
    echo -n "> " >&"${COPROC[1]}"
    read cmd <&"${COPROC[0]}"
    cmd="${cmd::-1}"

    case "$cmd" in
    !continue) break ;;
    *) eval "$cmd" >&"${COPROC[1]}" ;;
    esac
  done

  echo "#############|  End of DEBUG session |####################" >&"${COPROC[1]}"

  sleep 0.1
  kill "$COPROC_PID"
}
