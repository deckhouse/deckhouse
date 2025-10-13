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

const fs = require('fs');

const STATIC_MODULES = [
  'ci',
  'dependabot',
  'testing',
  'tools',
  'docs',
  'candi',
  'deckhouse-controller',
  'common',
  'registrypackages'
];

/**
 *
 * @param {string} dirPath
 * @returns {boolean}
 */
function isModule(dirPath) {
  /** @type {fs.Dirent[]} */
  const files = fs.readdirSync(dirPath, { withFileTypes: true });

  // Chart.yaml + images/ + hooks/
  let hasChartYaml = false;
  let hasImagesDir = false;
  let hasHooksDir = false;

  // Chart.yaml + openapi/
  let hasOpenapiDir = false;

  // go.mod + go.sum
  let hasGoMod = false;
  let hasGoSum = false;

  for (const file of files) {
    if (file.name === 'module.yaml' && file.isFile()) {
      return true;
    }

    if (file.name === 'Chart.yaml' && file.isFile()) {
      hasChartYaml = true;
    }

    if (file.name === 'openapi' && file.isDirectory()) {
      hasOpenapiDir = true;
    }

    if (file.name === 'images' && file.isDirectory()) {
      hasImagesDir = true;
    }

    if (file.name === 'hooks' && file.isDirectory()) {
      hasHooksDir = true;
    }

    if (file.name === 'go.mod' && file.isFile()) {
      hasGoMod = true;
    }

    if (file.name === 'go.sum' && file.isFile()) {
      hasGoSum = true;
    }
  }

  return (hasChartYaml && hasImagesDir && hasHooksDir) || (hasGoSum && hasGoMod) || (hasChartYaml && hasOpenapiDir);
}

/**
 * @description Walk by dirs and find modules
 * @param {string} root
 * @returns {string[]}
 */
function walk(root) {
  /** @type {string[]} */
  let result = [];

  /** @type {fs.Dirent[]} */
  const dirs = fs.readdirSync(root, { withFileTypes: true });

  sanitazeName = (name) => name.replace(/^\d+-/g, '');

  for (const dir of dirs) {
    const directoryPath = `${dir.parentPath}/${dir.name}`;
    if (dir.isDirectory()) {
      if (isModule(directoryPath)) {
        result.push(sanitazeName(dir.name));
      } else {
        result = result.concat(walk(directoryPath));
      }
    }
  }

  return result;
}

/**
 *
 * @param {string} root directory path
 * @returns {string[]} name of the modules
 */
function findIn(root = '.') {
  let result = walk(root).concat(STATIC_MODULES);

  result = result.filter((module) => module !== 'src');
  result.sort();

  return [...new Set(result)];
}

/**
 * test in console
 * node ./.github/scripts/js/parsers/modules-parser.js
 * or
 * node
 * node > const { findIn }= require('./.github/scripts/js/parsers/modules-parser.js')
 * node > findIn() or findIn('./modules')
 */
module.exports = {
  STATIC_MODULES,
  isModule,
  walk,
  findIn
};
