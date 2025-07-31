#!/usr/bin/env python3

# Copyright 2025 Flant JSC
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

import json
import os
import sys
import re

whitelist = [
    "base-for-go",
    "common-base",
    "common/shell-operator",
    "control-plane-manager/control-plane-manager.*",
    "control-plane-manager/kube-apiserver.*",
    "control-plane-manager/kube-controller-manager.*",
    "control-plane-manager/kube-scheduler.*",
    "deckhouse/webhook-handler",
    "dev-prebuild",
    "dev",
    "dev/install-standalone",
    "dev/install",
    "documentation/web",
    "e2e-opentofu-eks",
    "e2e-terraform",
    "images-digests",
    "kube-proxy/kube-proxy.*",
    "node-manager/bashible-apiserver",
    "prometheus/grafana-dashboard-provisioner",
    "registrypackages/kubeadm.*",
    "registrypackages/kubectl.*",
    "registrypackages/kubelet.*",
    "release-channel-version-prebuild",
    "tests-prebuild",
    "tests",
]

# Find and read build reports
editions = [i.removeprefix('build_report_') for i in os.listdir() if i.startswith('build_report_')]
print(f"Found editions: {editions}")
if len(editions) <= 1:
    print(f"Not enough editions to compare. Exit.")
    sys.exit()
reports = {}
unique_images = set()
for edition in editions:
    with open(f'build_report_{edition}/images_tags_werf.json') as file:
        reports[edition] = {k: v for (k, v) in json.load(file)['Images'].items() if v['Final']}
        unique_images.update(reports[edition].keys())

# Find which images have more than one unique digest
found = False
for image in unique_images:
    digests = set()
    for edition in editions:
        if image in reports[edition]:
            digests.add(reports[edition][image]['DockerImageDigest'])
    if len(digests) > 1:
        if not [image for pattern in whitelist if re.fullmatch(pattern, image)]:
            found = True
            print(f'Found differing image digests for image {image}:')
        else:
            print(f'Found differing image digests for image {image} (allowed to differ):')
        for edition in editions:
            if image in reports[edition]:
                print(f'\t{edition} digest: {reports[edition][image]['DockerImageDigest']}')

if found:
    sys.exit(1)
