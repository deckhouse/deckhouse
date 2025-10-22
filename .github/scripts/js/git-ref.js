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

/**
 * Helpers to work with Git refs.
 */


/**
 * Parse the Git ref.
 *
 * @param {string} ref — A Git ref (refs/heads/* or refs/tags/*)
 * @returns {object}
 */
const parseGitRef = (ref) => {
  let branchName = '';
  let tagName = '';
  let version = '';
  let majorMinor = '';
  let description = '';
  let isDeveloperTag = false;
  let refSlug = '';

  if (ref.startsWith('refs/heads')) {
    branchName = ref.replace('refs/heads/', '')
    refSlug = branchName
    if (branchName === 'main') {
      description = 'default branch';
    }

    const matches = fullMatchReleaseBranch(branchName)
    if (matches) {
      description = 'release branch';
      majorMinor = 'v'+matches[1]; // vX.Y
    }
  } else if (ref.startsWith('refs/tags/')) {
    tagName = ref.replace('refs/tags/', '')
    refSlug = tagName

    let matches = fullMatchReleaseTag(tagName)
    if (matches) {
      version = matches[0]; // vX.Y.Z
      majorMinor = matches[1]; // vX.Y.
      description = 'release tag';
    }

    // test-v1.32.1-0 to test before pushing a "real" tag.
    matches = fullMatchTestTag(tagName)
    if (matches) {
      version = 'v'+matches[0]; // vX.Y.Z
      majorMinor = 'v'+matches[1]; // vX.Y.
      description = 'test tag';
    }

    // dev-my-feature or pr-255-test.0
    if (/^(dev-|pr-)/.test(tagName)) {
      isDeveloperTag = true;
      description = 'developer tag';
    }
  }

  return {
    description,
    branchName,
    branchMajorMinor: branchName ? majorMinor : '',
    isBranch: !!branchName,
    isMain: branchName === 'main',
    isReleaseBranch: branchName.startsWith('release-') && !!majorMinor,
    tagName,
    tagVersion: tagName ? version : '',
    tagMajorMinor: tagName ? majorMinor : '',
    isTag: !!tagName,
    isDeveloperTag,
    ref,
    refSlug,
  };
};
module.exports.parseGitRef = parseGitRef;

// vX.Y.Z
const semVerReleaseTagNameFullMatch = /^(v\d+\.\d+)\.\d+$/
const semVerReleaseTagName = /v(\d+\.\d+)\.\d+/
// test-vX.Y.Z
const semVerTestTagNameFullMatch = /^test-v?(\d+\.\d+)\.\d+/
// release-X.Y
const releaseBranchNameFullMatch = /^release-(\d+\.\d+)$/


/**
 * @param {string} input — A string to test.
 * @returns {object|null}
 */
const matchReleaseTag = (input) => {
  if (!input) {
    return null
  }
  return input.match(semVerReleaseTagName);
}
module.exports.matchReleaseTag = matchReleaseTag

const fullMatchReleaseTag = (input) => {
  if (!input) {
    return null
  }
  return input.match(semVerReleaseTagNameFullMatch)
}
module.exports.fullMatchReleaseTag = fullMatchReleaseTag

const fullMatchTestTag = (input) => {
  if (!input) {
    return null
  }
  return input.match(semVerTestTagNameFullMatch)
}
module.exports.fullMatchTestTag = fullMatchTestTag

const fullMatchReleaseBranch = (input) => {
  if (!input) {
    return null
  }
  return input.match(releaseBranchNameFullMatch)
}
module.exports.fullMatchReleaseBranch = fullMatchReleaseBranch
