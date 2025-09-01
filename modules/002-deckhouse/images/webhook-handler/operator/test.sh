#!/bin/bash
echo "--- generate ---"
make generate

echo ""
echo "--- manifests ---"
make manifests

echo ""
echo "--- uninstall ---"
make uninstall

echo ""
echo "--- install ---"
make install
kubectl apply -f config/samples/deckhouse.io_v1alpha1_validationwebhook.yaml

echo ""
echo "--- run ---"
make run
