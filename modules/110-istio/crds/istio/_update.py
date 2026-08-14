#!/usr/bin/env python3

# Copyright 2026 Flant JSC
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

"""Split upstream Istio CRD bundles into the installed compatibility bundle."""
import argparse
from pathlib import Path
import shutil
import tempfile
import yaml

REQUIRED_SERVED_VERSIONS = {
    'authorizationpolicies.security.istio.io': {'v1', 'v1beta1'},
    'destinationrules.networking.istio.io': {'v1', 'v1alpha3', 'v1beta1'},
    'envoyfilters.networking.istio.io': {'v1alpha3'},
    'gateways.networking.istio.io': {'v1', 'v1alpha3', 'v1beta1'},
    'peerauthentications.security.istio.io': {'v1', 'v1beta1'},
    'proxyconfigs.networking.istio.io': {'v1beta1'},
    'requestauthentications.security.istio.io': {'v1', 'v1beta1'},
    'serviceentries.networking.istio.io': {'v1', 'v1alpha3', 'v1beta1'},
    'sidecars.networking.istio.io': {'v1', 'v1alpha3', 'v1beta1'},
    'telemetries.telemetry.istio.io': {'v1', 'v1alpha1'},
    'virtualservices.networking.istio.io': {'v1', 'v1alpha3', 'v1beta1'},
    'wasmplugins.extensions.istio.io': {'v1alpha1'},
    'workloadentries.networking.istio.io': {'v1', 'v1alpha3', 'v1beta1'},
    'workloadgroups.networking.istio.io': {'v1', 'v1alpha3', 'v1beta1'},
}
CONFIG = set(REQUIRED_SERVED_VERSIONS)
OPERATOR = {
    'istiooperators.install.istio.io',
    'istiocnis.sailoperator.io',
    'istiorevisions.sailoperator.io',
    'istiorevisiontags.sailoperator.io',
    'istios.sailoperator.io',
    'ztunnels.sailoperator.io',
}


def load(path):
    with open(path) as f:
        return [x for x in yaml.safe_load_all(f) if x]


def validate(objects):
    names = [x.get('metadata', {}).get('name') for x in objects]
    expected = CONFIG | OPERATOR
    if set(names) != expected or len(names) != len(expected):
        raise SystemExit(f'expected exactly {len(expected)} CRDs, got {sorted(names)}')
    for obj in objects:
        if obj.get('apiVersion') != 'apiextensions.k8s.io/v1' or obj.get('kind') != 'CustomResourceDefinition':
            raise SystemExit(f"{obj.get('metadata', {}).get('name')}: not a v1 CRD")
        versions = obj.get('spec', {}).get('versions', [])
        if sum(bool(v.get('storage')) for v in versions) != 1:
            raise SystemExit(f"{obj['metadata']['name']}: expected one storage version")
        name = obj['metadata']['name']
        if name in CONFIG:
            served = {version['name'] for version in versions if version.get('served')}
            missing = REQUIRED_SERVED_VERSIONS[name] - served
            if missing:
                raise SystemExit(
                    f'{name}: required served versions were removed: {sorted(missing)}; '
                    f'served versions: {sorted(served)}'
                )


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--check', action='store_true', help='validate the installed bundle')
    ap.add_argument('--istio-config')
    ap.add_argument('--istio-operator')
    ap.add_argument('--sail-dir')
    args = ap.parse_args()
    here = Path(__file__).parent
    if args.check:
        objects = []
        for path in sorted(here.glob('*.yaml')):
            docs = load(path)
            if len(docs) != 1:
                raise SystemExit(f'{path}: expected exactly one YAML object')
            objects += docs
        validate(objects)
        return
    if not all((args.istio_config, args.istio_operator, args.sail_dir)):
        ap.error('--istio-config, --istio-operator, and --sail-dir are required unless --check is used')
    objects = load(args.istio_config) + load(args.istio_operator)
    for path in sorted(Path(args.sail_dir).glob('sailoperator.io_*.yaml')):
        objects += load(path)
    validate(objects)
    with tempfile.TemporaryDirectory(dir=here) as tmp:
        target = Path(tmp)
        for obj in objects:
            (target / (obj['metadata']['name'] + '.yaml')).write_text(yaml.safe_dump(obj, sort_keys=False))
        for old in here.glob('*.yaml'):
            old.unlink()
        for new in target.glob('*.yaml'):
            shutil.move(new, here / new.name)

if __name__ == '__main__':
    main()
