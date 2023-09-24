{{- define "bootstrap_script" }}
#!/usr/bin/env bash
set -Eeuo pipefail

function detect_bundle() {
  {{- .Files.Get "candi/bashible/detect_bundle.sh" | nindent 2 }}
}

function check_python() {
  if command -v python3 >/dev/null 2>&1; then
    python_binary="python3"
    script="$(p3_script)"
    return 0
  fi
  if command -v python2 >/dev/null 2>&1; then
    python_binary="python2"
    script="$(p2_script)"
    return 0
  fi
  if command -v python >/dev/null 2>&1; then
    python_binary="python"
    if python --version | grep -q "Python 3"; then
      script="$(p3_script)"
      return 0
    fi
    if python --version | grep -q "Python 2"; then
      script="$(p2_script)"
      return 0
    fi
  fi
  echo "Python not found, exiting..."
  return 1
}

function p2_script() {
  cat - <<EOF
import sys
import json
import urllib2
import ssl
request = urllib2.Request(sys.argv[1], headers={'Authorization': 'Bearer ' + sys.argv[2]})
response = urllib2.urlopen(request, context=ssl._create_unverified_context())
data = json.loads(response.read())
sys.stdout.write(data["bootstrap"])
EOF
}

function p3_script() {
  cat - <<EOF
import sys
import requests
import json
response = requests.get(sys.argv[1], headers={'Authorization': 'Bearer ' + sys.argv[2]}, verify='/var/lib/bashible/ca.crt')
data = json.loads(response.content)
sys.stdout.write(data["bootstrap"])
EOF
}

function get_phase2() {
  bundle="$(detect_bundle)"
  check_python
  bootstrap_bundle_name="$bundle.{{ .nodeGroupName }}"
  token="$(</var/lib/bashible/bootstrap-token)"
  while true; do
    for server in {{ .apiserverEndpoints | join " " }}; do
      url="https://$server/apis/bashible.deckhouse.io/v1alpha1/bootstrap/$bootstrap_bundle_name"
      if $("$python_binary" - "$url" "$token" <<< "$script"); then
        return 0
      fi
      >&2 echo "failed to get bootstrap $bootstrap_bundle_name from $url"
    done
    sleep 10
  done
}

get_phase2 | bash
{{- end }}
