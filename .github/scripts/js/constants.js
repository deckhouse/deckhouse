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
    'e2e/run/gcp',
    'e2e/run/openstack',
    'e2e/run/vsphere',
    'e2e/run/yandex-cloud',
    'e2e/run/static'
  ],
  'issue-release': 'issue/release',
  'ok-to-test': 'status/ok-to-test',
  // prettier-ignore
  'deploy-web': [
    'deploy/web/test',
    'deploy/web/stage'
  ],
  'edition': [
    'edition/ce',
    'edition/ee'
  ]
};
module.exports.knownLabels = labels;

const slashCommands = {
  deploy: [
    'deploy/alpha',
    'deploy/beta',
    'deploy/early-access',
    'deploy/stable',
    'deploy/rock-solid'
  ],
  suspend: [
    'suspend/alpha',
    'suspend/beta',
    'suspend/early-access',
    'suspend/stable',
    'suspend/rock-solid'
  ],
};
module.exports.knownSlashCommands = slashCommands;

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
  'gcp',
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

const criNames = [
  'Containerd',
  'Docker',
];
module.exports.knownCRINames = criNames;

const kubernetesVersions = [
  '1.19',
  '1.20',
  '1.21',
  '1.22',
  '1.23',
];
module.exports.knownKubernetesVersions = kubernetesVersions;

module.exports.e2eDefaults = {
  criName: 'Containerd',
  kubernetesVersion: '1.21',
}

const editions = [
  'CE',
  'EE',
  'FE'
];
module.exports.knownEditions = editions;
