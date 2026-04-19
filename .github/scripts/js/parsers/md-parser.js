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

const marked = require('marked');

/**
 *
 * @param {string} markdownString
 * @returns {marked.TokensList}
 */
function parseMarkdown(markdownString) {
  const tokens = marked.lexer(markdownString);

  return tokens;
}

/**
 *
 * @param {marked.TokensList} tokens - Array of markdown tokens
 * @param {string} header - The header to search for
 * @param {string} searchField - The name of the key
 * @param {string} searchValue - Key value
 * @returns {string|null} - Returns text following the specified header in the 'changes' code block, if found
 */
function findSectionInMarkdown(tokens, header, searchField, searchValue) {
  const headerIndex = tokens.findIndex((token) => token.type === 'heading' && token.text.toLowerCase() === header.toLowerCase());

  if (headerIndex === -1) return null;

  // looking for the first token after the header with the specified field and value
  for (let i = headerIndex + 1; i < tokens.length; i++) {
    const token = tokens[i];

    // stop the search at the next title
    if (token.type === 'heading') break;

    if (token[searchField] === searchValue) {
      return token.text;
    }
  }

  return null;
}

module.exports = {
  parseMarkdown,
  findSectionInMarkdown
};
