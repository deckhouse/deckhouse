//@ts-check

// Labels available for pull requests.
const labels = {
  // Skip validations.
  'skip/no-cyrillic-validation': { type: 'skip-validation', validation_name: 'no_cyrillic' },
  'skip/documentation-validation': { type: 'skip-validation', validation_name: 'doc_changes' },
  'skip/copyright-validation': { type: 'skip-validation', validation_name: 'copyright' },
  'skip/markdown-validation': { type: 'skip-validation', validation_name: 'markdown' },
  'skip/actionlint': { type: 'skip-validation', validation_name: 'actionlint' },
  'skip/e2e': { type: 'skip-validation', validation_name: 'e2e_skip' },

  // E2E
  'e2e/run/aws': { type: 'e2e-run', provider: 'aws' },
  'e2e/run/azure': { type: 'e2e-run', provider: 'azure' },
  'e2e/run/gcp': { type: 'e2e-run', provider: 'gcp' },
  'e2e/run/openstack': { type: 'e2e-run', provider: 'openstack' },
  'e2e/run/vsphere': { type: 'e2e-run', provider: 'vsphere' },
  'e2e/run/yandex-cloud': { type: 'e2e-run', provider: 'yandex-cloud' },
  'e2e/run/static': { type: 'e2e-run', provider: 'static' },

  // E2E: use CRI
  'e2e/use/cri/docker': { type: 'e2e-use', cri: 'Docker' },
  'e2e/use/cri/containerd': { type: 'e2e-use', cri: 'Containerd' },

  // E2E: use Kubernetes version
  'e2e/use/k8s/1.20': { type: 'e2e-use', ver: '1.20' },
  'e2e/use/k8s/1.21': { type: 'e2e-use', ver: '1.21' },
  'e2e/use/k8s/1.22': { type: 'e2e-use', ver: '1.22' },
  'e2e/use/k8s/1.23': { type: 'e2e-use', ver: '1.23' },
  'e2e/use/k8s/1.24': { type: 'e2e-use', ver: '1.24' },

  // Allow running workflows for external PRs.
  'status/ok-to-test': { type: 'ok-to-test' },

  // Deploy documentation and site to test or stage.
  'deploy/web/test': { type: 'deploy-web', env: 'test' },
  'deploy/web/stage': { type: 'deploy-web', env: 'stage' },

  // Edition for build-and-test workflow
  'edition/ce': { type: 'edition', edition: 'CE' },
  'edition/ee': { type: 'edition', edition: 'EE' }
};
module.exports.knownLabels = labels;

// Label to detect if issue is a release issue.
const releaseIssueLabel = 'issue/release';
module.exports.releaseIssueLabel = releaseIssueLabel;


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
   * Search for known label name using its type and property:
   * - search by provider property for e2e-run labels
   * - search by env property for deploy-web labels
   *
   * @param {object} inputs
   * @param {string} inputs.labelType
   * @param {string} inputs.labelSubject
   * @returns {string}
   */
  findLabel: ({ labelType, labelSubject }) => {
    return (Object.entries(labels).find(([name, info]) => {
      if (info.type === labelType) {
        if (labelType === 'e2e-run') {
          return info.provider === labelSubject;
        }
        if (labelType === 'deploy-web') {
          return info.env === labelSubject;
        }
      }
      return false;
    }) || [''])[0];
  }
};

// Providers for e2e tests.
const providers = Object.entries(labels)
  .filter(([name, info]) => info.type === 'e2e-run')
  .map(([name, info]) => info.provider)
  .sort();
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

const criNames = Object.entries(labels)
  .filter(([name, info]) => info.type === 'e2e-use' && !!info.cri)
  .map(([name, info]) => info.cri);
module.exports.knownCRINames = criNames;

const kubernetesVersions = Object.entries(labels)
  .filter(([name, info]) => info.type === 'e2e-use' && !!info.ver)
  .map(([name, info]) => info.ver)
  .sort();
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
