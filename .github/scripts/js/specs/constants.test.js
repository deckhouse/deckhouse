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
  expect(knownCRINames).toContain('Docker')
})

test('knownKubernetesVersions', () => {
  expect(knownKubernetesVersions).toContain('1.19')
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
