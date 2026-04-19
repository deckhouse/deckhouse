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

// required yaml https://www.npmjs.com/package/yaml
const YAML = require('yaml');

/**
 * @param {string} blockText
 * @returns {string}
 */
function processBlock(blockText) {
  const lines = blockText.split('\n');
  const targetFields = ['summary', 'impact']; // fields being processed
  const unsafeChars = ['[', '{'];
  const processedLines = [];

  for (const line of lines) {
    let processedLine = line;

    for (const field of targetFields) {
      if (processedLine.startsWith(`${field}:`)) {
        const colonIndex = processedLine.indexOf(':');
        const key = processedLine.slice(0, colonIndex + 1);
        let valuePart = processedLine.slice(colonIndex + 1).trim();

        // Check if the value is wrapped in quotes
        const isQuoted =
          (valuePart.startsWith('"') && valuePart.endsWith('"')) || (valuePart.startsWith("'") && valuePart.endsWith("'"));

        if (!isQuoted && valuePart.length > 0) {
          const firstChar = valuePart[0];
          if (unsafeChars.includes(firstChar)) {
            // Escape double quotes inside the value
            valuePart = valuePart.replace(/"/g, '\\"');
            processedLine = `${key} "${valuePart}"`;
          }
        }
        break; // don't check other fields after a match
      }
    }

    processedLines.push(processedLine);
  }

  return processedLines.join('\n');
}

/**
 * @param {object} block
 * @param {number} index
 * @param {string[]} allowedSections
 * @returns {boolean}
 */
function validateYaml(block, index, allowedSections) {
  if (
    block.section === undefined ||
    block.section === null ||
    block.section.length === 0 ||
    block.section === '<kebab-case of a module name> | <1st level dir in the repo>'
  ) {
    throw new Error(`'section' is required and must be a non-empty string and allowed section in block ${index}`);
  }

  const blockSections = block.section.split(',').map((section) => section.trim());
  blockSections.forEach((section) => {
    if (!allowedSections.includes(section)) {
      console.log('Allowed sections:', allowedSections.join(', '));
      throw new Error(`section '${section}' is not an allowed section in block ${index}`);
    }
  });

  if (
    block.type === undefined ||
    block.type === null ||
    block.type.length === 0 ||
    !['fix', 'feature', 'chore'].includes(block.type)
  ) {
    throw new Error(`'type' must be one of type: fix, feature, chore. In block ${index}`);
  }

  if (
    block.summary === undefined ||
    block.summary === null ||
    block.summary.length === 0 ||
    block.summary === '<ONE-LINE of what effectively changes for a user>'
  ) {
    throw new Error(`'summary' is required and must be a non-empty string in block ${index}`);
  }

  if (
    typeof block.impact_level === 'string' &&
    block.impact_level.length > 0 &&
    !['default', 'high', 'low'].includes(block.impact_level)
  ) {
    throw new Error(`'impact_level' must be one of levels: default, high, low. In block ${index}`);
  }

  if (block.impact_level === 'high' && (block.impact === undefined || block.impact === null || block.impact.length === 0)) {
    throw new Error(`'impact' is required when 'impact_level' is 'high' in block ${index}`);
  }

  if (
    typeof block.impact === 'string' &&
    block.impact.length > 0 &&
    block.impact === '<what to expect for users, possibly MULTI-LINE>, required if impact_level is high ↓'
  ) {
    throw new Error(`'impact' is required and must be a non-empty string in block ${index}`);
  }

  return true;
}

/**

 * @param {string} changelogEntries
 * @param {string[]} allowedSections
 */
function validatePullRequestChangelog(changelogEntries, allowedSections) {
  let changesBlocks = changelogEntries.split('---');
  try {
    changesBlocks.forEach((changeBlock, idx) => {
      const processedBlock = processBlock(changeBlock.trim());
      const parsed = YAML.parse(processedBlock);
      console.log(parsed);
      validateYaml(parsed, idx + 1, allowedSections);
    });

    console.log('Changes is valid');
  } catch (error) {
    throw error;
  }
}

module.exports.validatePullRequestChangelog = validatePullRequestChangelog;
