#!/bin/sh

while true; do
  resp_code="$(curl -s -o /dev/null -w "%{http_code}" --max-time 3 "https://$KUBE_SCHEDULER_IP:$KUBE_SCHEDULER_PORT/metrics" -k --cert /etc/ssl/private/tls.crt --key /etc/ssl/private/tls.key)"

  if [[ "$resp_code" == "403" ]]; then
    kube_scheduler_pid="$(ss -nltp4 | grep "$KUBE_SCHEDULER_IP:$KUBE_SCHEDULER_PORT" | sed -E 's/.*pid=([0-9]+).*/\1/')" && \
    kill $kube_scheduler_pid && \
    echo "$(date -u) kube-scheduler[$kube_scheduler_pid] slapped!"
  fi

  sleep 60
done
