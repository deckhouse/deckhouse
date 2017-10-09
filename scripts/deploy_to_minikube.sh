#!/bin/bash

set -e

dapp dimg build --dev
dapp dimg push :minikube --dev
dapp kube deploy :minikube --dev --namespace antiopa --set "global.env=minikube"
