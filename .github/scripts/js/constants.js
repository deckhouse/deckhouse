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

//@ts-check

const skipE2eLabel = 'skip/e2e';
const abortFailedE2eCommand = '/e2e/abort';
module.exports.skipE2eLabel = skipE2eLabel;
module.exports.abortFailedE2eCommand = abortFailedE2eCommand;

// Labels available for pull requests.
const labels = {
  // Skip validations.
  'skip/no-cyrillic-validation': { type: 'skip-validation', validation_name: 'no_cyrillic' },
  'skip/documentation-validation': { type: 'skip-validation', validation_name: 'doc_changes' },
  'skip/copyright-validation': { type: 'skip-validation', validation_name: 'copyright' },
  'skip/grafana-dashboard': { type: 'skip-validation', validation_name: 'grafana_dashboard' },
  'skip/markdown-validation': { type: 'skip-validation', validation_name: 'markdown' },
  'skip/actionlint': { type: 'skip-validation', validation_name: 'actionlint' },
  'skip/release-requirements': { type: 'skip-validation', validation_name: 'release_requirements' },

  // E2E
  'e2e/run/aws': { type: 'e2e-run', provider: 'aws' },
  'e2e/run/azure': { type: 'e2e-run', provider: 'azure' },
  'e2e/run/eks': { type: 'e2e-run', provider: 'eks' },
  'e2e/run/gcp': { type: 'e2e-run', provider: 'gcp' },
  'e2e/run/openstack': { type: 'e2e-run', provider: 'openstack' },
  'e2e/run/vsphere': { type: 'e2e-run', provider: 'vsphere' },
  'e2e/run/yandex-cloud': { type: 'e2e-run', provider: 'yandex-cloud' },
  'e2e/run/static': { type: 'e2e-run', provider: 'static' },

  // E2E: use Kubernetes version
  'e2e/use/k8s/1.25': { type: 'e2e-use', ver: '1.25' },
  'e2e/use/k8s/1.26': { type: 'e2e-use', ver: '1.26' },
  'e2e/use/k8s/1.27': { type: 'e2e-use', ver: '1.27' },
  'e2e/use/k8s/1.28': { type: 'e2e-use', ver: '1.28' },
  'e2e/use/k8s/1.29': { type: 'e2e-use', ver: '1.29' },
  'e2e/use/k8s/automatic': { type: 'e2e-use', ver: 'Automatic' },

  // Allow running workflows for external PRs.
  'status/ok-to-test': { type: 'ok-to-test' },

  // Deploy documentation and site to test or stage.
  'deploy/web/test': { type: 'deploy-web', env: 'test' },
  'deploy/web/stage': { type: 'deploy-web', env: 'stage' },

  // Edition for build-and-test workflow
  'edition/ce': { type: 'edition', edition: 'CE' },
  'edition/ee': { type: 'edition', edition: 'EE' },
  'edition/be': { type: 'edition', edition: 'BE' },
  'edition/se': { type: 'edition', edition: 'SE' }
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

        return true;
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
}

const editions = [
  'CE',
  'EE',
  'FE',
  'BE',
  'SE'
];
module.exports.knownEditions = editions;
