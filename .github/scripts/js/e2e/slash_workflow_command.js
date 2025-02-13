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

const {abortFailedE2eCommand} = require("../constants");

/**
 * Try parse e2e abort arguments
 * @param {object} inputs
 * @param {object} inputs.core - A reference to the '@actions/core' package.
 * @param {object} inputs.context - A reference to context https://github.com/actions/toolkit/blob/main/packages/github/src/context.ts#L6
 * @param {string[]} inputs.argv - array of slash command argv[0] is commnad
 * @return {object}
 */
function tryParseAbortE2eCluster({argv, context, core}){
  const command = argv[0];
  if (command !== abortFailedE2eCommand) {
    return null;
  }

  // example
  // /e2e/abort static;Static;containerd;1.21 3318607912 3318607912-1-con-1-21
  // explain:
  // /e2e/abort - command
  // static;Static;containerd;1.21 - run parameters (provider;layout;cri;k8s version)
  // 3318607912 - run id (needs for get artifact)
  // 3318607912-1-con-1-21 - cluster prefix (needs for run dhctl bootstrap-phase abort command)
  // /sys/deckhouse-oss/install:pr2896 - install image path: for run necessary installer
  // user@127.0.0.1 - [additional] connection string, needs for fully bootstrapped cluster, but e2e was failed.
  //                  we  need it for destroy
  if (argv.length < 5) {
    return {err: 'clean failed e2e cluster should have 4 arguments'};
  }

  const ranForSplit = argv[1].split(';').map(v => v.trim()).filter(v => !!v);
  if (ranForSplit.length !== 4) {
    return {err: '"ran parameters" argument should split on 4 parts'};
  }

  const run_id = argv[2];
  const cluster_prefix = argv[3];
  const installer_image_path = argv[4];
  let sshConnectStr = '';
  if (argv.length === 6) {
    sshConnectStr = argv[5] || '';
  }

  const prNumber = context.payload.issue.number;

  core.debug(`pull request info: ${JSON.stringify({prNumber, installer_image_path})}`);

  const provider = ranForSplit[0];
  const layout = ranForSplit[1];
  const cri = ranForSplit[2];
  const k8s_version = ranForSplit[3];
  const edition = 'fe';
  const k8sSlug = k8s_version.replace('.', '_');
  const state_artifact_name = `failed_cluster_state_${provider}_${cri}_${k8sSlug}`;
  const test_config = JSON.stringify({ cri: cri, ver: k8s_version, edition: edition })

  const inputs = {
    run_id,
    state_artifact_name,
    cluster_prefix,
    installer_image_path,
    ssh_master_connection_string: sshConnectStr,

    layout,
    test_config,
    issue_number: prNumber.toString(),
  };

  core.debug(`e2e abort inputs: ${JSON.stringify(inputs)}`)

  return {
    isDestroyFailedE2e: true,
    workflow_id: `e2e-abort-${provider}.yml`,
    targetRef: 'refs/heads/main',
    inputs,
  }
}


module.exports = {
  tryParseAbortE2eCluster,
}
