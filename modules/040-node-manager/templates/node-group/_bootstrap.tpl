{{- define "bootstrap_script" }}
#!/usr/bin/env bash
set -Eeuo pipefail

function detect_bundle() {
  {{- .Files.Get "candi/bashible/detect_bundle.sh" | nindent 2 }}
}

function p2_script() {
  cat - <<EOF
import os
import json
import urllib2
import ssl
request = urllib2.Request(os.getenv('url', ''), headers={'Authorization': 'Bearer ' + os.getenv('token', '')})
response = urllib2.urlopen(request, context=ssl._create_unverified_context())
data = json.loads(response.read())
print(data["bootstrap"])
EOF
}

function p3_script() {
  cat - <<EOF
import os
import requests
import json
response = requests.get(os.getenv('url', ''), headers={'Authorization': 'Bearer ' + os.getenv('token', '')}, verify='/var/lib/bashible/ca.crt')
data = json.loads(response.content)
print(data["bootstrap"])
EOF
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

function get_phase2() {
  check_python
  local bootstrap_bundle_name="$bundle.{{ .nodeGroupName }}"
  export token="$(</var/lib/bashible/bootstrap-token)"
  while true; do
    for server in {{ .apiserverEndpoints | join " " }}; do
      export url="https://$server/apis/bashible.deckhouse.io/v1alpha1/bootstrap/$bootstrap_bundle_name"
      if phase2=$("$python_binary" <<< "$script"); then
        cat - <<< $phase2
        return 0
      else
        >&2 echo "failed to get bootstrap $bootstrap_bundle_name from $url"
      fi
    done
    sleep 10
  done
}

bundle="$(detect_bundle)"
get_phase2 | bash
{{- end }}
