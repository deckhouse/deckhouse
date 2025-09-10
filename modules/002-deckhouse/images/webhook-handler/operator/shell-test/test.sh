#!/bin/bash
rm -rf hooks

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
# kubectl apply -f config/samples/v1alpha1_validationwebhook.yaml
kubectl apply -f config/samples/v1alpha1_conversionwebhook.yaml

echo ""
echo "--- run ---"
make run
