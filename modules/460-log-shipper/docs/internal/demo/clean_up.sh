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

kubectl -n kube-system delete secret audit-policy
kubectl delete clusterloggingconfig all-pods audit-logs
kubectl delete clusterlogdestination all-pods audit-logs
kubectl delete grafanaadditionaldatasource loki

helm uninstall loki
kubectl delete ns loki-test
