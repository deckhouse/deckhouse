# Turns the controller-gen output for NodeConfig into the CRD the module ships:
# drops the controller-gen version annotation, adds the Deckhouse heritage
# labels every module CRD carries, and stamps x-kubernetes-sensitive-data onto
# status.maintenanceToken.
#
# That last marker is what makes the API strip the token from every
# get/list/watch answer that lacks the nodeconfigs/sensitive subresource, which
# nodes are deliberately not granted. controller-gen has no marker for it, so it
# is applied here. Fails loudly when the property is not found: a marker that
# silently stopped being applied publishes a root-equivalent credential to every
# node in the cluster.
#
# awk rather than a YAML editor, which would reindent the whole generated file.

/^  annotations:$/ { inAnnotations = 1; next }
inAnnotations && /^    controller-gen\.kubebuilder\.io\/version:/ { next }
inAnnotations { inAnnotations = 0 }

/^  name: nodeconfigs\.internal\.deckhouse\.io$/ {
    print
    print "  labels:"
    print "    heritage: deckhouse"
    print "    module: node-manager"
    named = 1
    next
}

{ print }

/^[[:space:]]*maintenanceToken:[[:space:]]*$/ { inToken = 1; next }
inToken && /^[[:space:]]*type: string[[:space:]]*$/ {
    match($0, /^[[:space:]]*/)
    print substr($0, 1, RLENGTH) "x-kubernetes-sensitive-data: true"
    inToken = 0
    marked = 1
}

END {
    if (!named) {
        print "nodeconfig-crd: the CRD metadata.name was not found in the generated CRD" > "/dev/stderr"
        exit 1
    }
    if (!marked) {
        print "nodeconfig-crd: status.maintenanceToken was not found in the generated CRD" > "/dev/stderr"
        exit 1
    }
}
