// Copyright 2022 Flant JSC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

const {
  knownProviders,
  knownKubernetesVersions,
  knownCRINames,
  labelsSrv
} = require('../constants');

test('knownProviders', () => {
  expect(knownProviders).toContain('azure')
  expect(knownProviders).toContain('aws')
})

test('knownCRINames', () => {
  expect(knownCRINames).toContain('Containerd')
})

test('knownKubernetesVersions', () => {
  expect(knownKubernetesVersions).toContain('1.20')
  expect(knownKubernetesVersions).toContain('1.23')
})

test('e2e/run/azure', () => {
  const l = labelsSrv.findLabel({labelType: 'e2e-run', labelSubject: 'azure'})
  expect(l).toBe('e2e/run/azure')
});

test('deploy/web/stage', () => {
  const l = labelsSrv.findLabel({labelType: 'deploy-web', labelSubject: 'stage'})
  expect(l).toBe('deploy/web/stage')
});

test('unknown label', () => {
  let l = labelsSrv.findLabel({labelType: 'e2e-run-', labelSubject: 'azure'})
  expect(l).toBe('')
  l = labelsSrv.findLabel({labelType: 'e2e-run', labelSubject: 'azure-'})
  expect(l).toBe('')
  l = labelsSrv.findLabel({labelType: '', labelSubject: ''})
  expect(l).toBe('')
});
