// Copyright 2025 Flant JSC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/**
 * @example await sleep(1000 * 30); // 30 seconds
 * @param {number} ms
 */
function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * @example isReleaseBranch('release-1.67')
 * @param {string} branchName
 * @returns {boolean}
 */
function isReleaseBranch(branchName) {
  const regex = /^release-(\d+)\.(\d+)$/;
  return regex.test(branchName);
}

module.exports = {
  sleep,
  isReleaseBranch
};
