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
const fs = require('fs');

// How to test:
//  $ node
//  > const { find } = require('./.github/scripts/js/changelog-find-sections.js')
//  > find([".", "modules", "ee/modules", "ee/fe/modules"], ["^\\.", "CHANGELOG", "^ee$", "^modules$"])
module.exports.find = find;

/**
 * Find all supported section names for changelog
 * @param {string[]} roots      the array of directories, e.g. [".", "modules", "ee/modules", "ee/fe/modules"]
 * @param {string[]} exclusions the array of sections to exclude, e.g. ["^\\.", "CHANGELOG", "ee", "modules"]
 * @returns the array of sections
 * call([".", "modules", "ee/modules", "ee/fe/modules"], ["^\\.", "CHANGELOG", "^ee$", "^modules$"])
 */
function find(roots = [], exclusions = []) {
  const foundSections = roots.flatMap((root) => getSections(root, exclusions));
  const uniqSections = new Set(foundSections);
  return Array.from(uniqSections).sort();
}

// getSections returns sections excluding items with given regexes
function getSections(root, exclusions = []) {
  // trim numbers
  const sections = getSubdirs(root).map((name) => name.replace(/^\d+-/g, ''));

  if (exclusions.length == 0) {
    return sections;
  }

  const shouldExclude = (name) => exclusions.some((pat) => new RegExp(pat).test(name));
  return sections.filter((s) => !shouldExclude(s));
}

// getSubdirs returns dir names in a given root dir
function getSubdirs(root) {
  return fs
    .readdirSync(root, {withFileTypes: true})
    .filter((d) => d.isDirectory())
    .map((d) => d.name);
}