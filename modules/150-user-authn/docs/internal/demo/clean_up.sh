#!/bin/sh

# Copyright 2022 Flant JSC
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

kubectl delete dexprovider openldap-demo --ignore-not-found
kubectl delete dexprovider corp-ldap --ignore-not-found
kubectl delete user openldap-demo --ignore-not-found
kubectl delete ns openldap-demo --ignore-not-found

# Kerberos demo resources in d8-user-authn
kubectl -n d8-user-authn delete pod ldap-tools --ignore-not-found
kubectl -n d8-user-authn delete deploy kdc --ignore-not-found
kubectl -n d8-user-authn delete svc kdc --ignore-not-found
kubectl -n d8-user-authn delete configmap kdc-config --ignore-not-found
kubectl -n d8-user-authn delete secret dex-kerberos-test --ignore-not-found
kubectl -n d8-user-authn delete deploy openldap --ignore-not-found
kubectl -n d8-user-authn delete svc openldap --ignore-not-found
kubectl -n d8-user-authn delete oauth2client spnego-test --ignore-not-found
kubectl delete clusterauthorizationrule john-superadmin --ignore-not-found
