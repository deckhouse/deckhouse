#!/bin/bash
echo "--- generate ---"
make generate

echo ""
echo "--- manifests ---"
make manifests

echo ""
echo "--- install ---"
make install

echo ""
echo "--- run ---"
make run
