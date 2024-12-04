# Copyright 2024 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

function kubectl_exec() {
  kubectl --request-timeout 60s --kubeconfig=/etc/kubernetes/kubelet.conf ${@}
}

check_python
function fetch-local-labels() {
  cat - <<EOF | $python_binary
import re
import glob
import sys
import os
import json

def validate(string, is_key = True):
    if len(string) > 63:
        return False
    if is_key:
        pattern = re.compile("^(?:(?:(?:[a-z0-9][a-z0-9-]+)\.)+[a-z0-9]+/)*[A-Za-z0-9][A-Za-z0-9-._]*$")
    else:
        pattern = re.compile("^[A-Za-z0-9][A-Za-z0-9-._]*$")
    if pattern.fullmatch(string):
        return True
    return False

def fetch_labels(fileglob, valid = True):
    files = glob.glob(fileglob, recursive=True)
    labels = dict()
    for f in files:
        if os.path.isfile(f):
            with open(f) as file:
                flines = [line.rstrip() for line in file]
                for l in flines:
                    label = l.split('=')
                    if len(label) == 2:
                        if valid:
                            if validate(label[0]) and validate(label[1], False):
                                labels[label[0]] = label[1]
                        else:
                            labels[label[0]] = label[1]
    return labels

def print_labels(labels):
    label_string = ""
    for key in labels:
        label_string = label_string + key + "=" + labels[key] + " "
    return label_string.rstrip()

def get_removed(d1, d2):
    d1_keys = set(d1.keys())
    d2_keys = set(d2.keys())
    shared_keys = d1_keys.intersection(d2_keys)
    removed = d1_keys - d2_keys
    return removed

labels = fetch_labels("$1" + "/**", True)

if "$2" == "add":
    format = "$4"
    if format == "json":
       print(json.dumps(labels))
    else: 
        print(print_labels(labels))
if "$2" == "delete":
    try:
        node_labels = json.loads('$3')
        removed = get_removed(node_labels, labels)
        for k in removed:
            print("{}-".format(k))
    except:
        print("To use delete pass json as third argument")
        sys.exit(1)
EOF
}

LABEL_DIRECTORY_PATH=/var/lib/node_labels
mkdir -p $LABEL_DIRECTORY_PATH

LABELS_FROM_ANNOTATION="$( kubectl_exec get no "$D8_NODE_HOSTNAME" -o json |jq -r '.metadata.annotations."node.deckhouse.io/last-applied-local-labels"' )"

if [[ $LABELS_FROM_ANNOTATION == "null" ]]
  then
    LABELS_FROM_ANNOTATION='{}'
fi

LABELS="$( fetch-local-labels "$LABEL_DIRECTORY_PATH" add)"
LABLES_TO_REMOVE="$( fetch-local-labels "$LABEL_DIRECTORY_PATH" delete "$LABELS_FROM_ANNOTATION")"
LABELS_ANNOTATION="$( fetch-local-labels "$LABEL_DIRECTORY_PATH" add "$LABELS_FROM_ANNOTATION" json )"

if [[ $LABLES_TO_REMOVE ]]
  then
    for label in $LABLES_TO_REMOVE; do
        kubectl_exec label node "${D8_NODE_HOSTNAME}" "$label"
    done
fi

if [[ -z $LABELS ]]
  then
    # No labels to apply, exit 0
    exit 0
  else
    # Apply labels to node
    kubectl_exec label node "${D8_NODE_HOSTNAME}" "${LABELS}" --overwrite
    kubectl --request-timeout 60s --kubeconfig=/etc/kubernetes/kubelet.conf annotate node "${D8_NODE_HOSTNAME}" --overwrite node.deckhouse.io/last-applied-local-labels="${LABELS_ANNOTATION}"
fi