//@ts-check

// Labels available for issues and pull requests.
const labels = {
  // prettier-ignore
  'skip-validation': [
    'skip/no-cyrillic-validation',
    'skip/documentation-validation',
    'skip/copyright-validation'
  ],
  e2e: [
    'e2e/run/aws',
    'e2e/run/azure',
    'e2e/run/gce',
    'e2e/run/openstack',
    'e2e/run/vsphere',
    'e2e/run/yandex-cloud',
    'e2e/run/static'
  ],
  'issue-release': 'issue/release',
  deploy: [
    'deploy/deckhouse/alpha',
    'deploy/deckhouse/beta',
    'deploy/deckhouse/early-access',
    'deploy/deckhouse/stable',
    'deploy/deckhouse/rock-solid'
  ],
  suspend: [
    'suspend/deckhouse/alpha',
    'suspend/deckhouse/beta',
    'suspend/deckhouse/early-access',
    'suspend/deckhouse/stable',
    'suspend/deckhouse/rock-solid'
  ],
  // prettier-ignore
  'deploy-web': [
    'deploy/web/test',
    'deploy/web/stage'
  ]
};

module.exports.knownLabels = labels;

module.exports.labelsSrv = {
  /**
   * Search for known label name using label type as prefix and subject as suffix.
   *
   * @param {object} inputs
   * @param {string} inputs.labelType
   * @param {string} inputs.labelSubject
   * @returns {string}
   */
  findLabel: ({ labelType, labelSubject }) => {
    const suffix = '/' + labelSubject.toLowerCase();
    for (const label of labels[labelType]) {
      if (label.endsWith(suffix)) {
        return label;
      }
    }
    return '';
  }
};

// Providers for e2e tests.
const providers = [
  //
  'aws',
  'gce',
  'azure',
  'openstack',
  'yandex-cloud',
  'vsphere',
  'static'
];

module.exports.knownProviders = providers;

// Channels available for deploy.
const channels = [
  //
  'alpha',
  'beta',
  'early-access',
  'stable',
  'rock-solid'
];

module.exports.knownChannels = channels;
