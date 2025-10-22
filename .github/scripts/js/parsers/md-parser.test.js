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

const { parseMarkdown, findSectionInMarkdown } = require('./md-parser');

describe('parseMarkdown', () => {
  const testContent = `
## Description
Added validation of the changes block

## Why do we need it, and what problem does it solve?
disallow merging if change block is invalid

## Why do we need it in the patch release (if we do)?

## What is the expected result?
Invalid PRs will not be merged

## Checklist
- [ ] The code is covered by unit tests.
- [ ] e2e tests passed.
- [ ] Documentation updated according to the changes.
- [ ] Changes were tested in the Kubernetes cluster manually.

## Changelog entries

\`\`\`changes
section: ci
type: chore
summary: Invalid PRs will not be merged
impact: disallow merging if change block is invalid
impact_level: low
\`\`\`

<!---
\`impact_level: default\` adds to changelog as usual, this is the default that can be omitted
\`impact_level: high\`    something important for users, the impact will be copied to "Know Before Update" section
\`impact_level: low\`     omitted in changelog YAML; note there is \`type:chore\` for chores

Tip for the section field:

- <kebab-case of a module>, e.g. "cloud-provider-aws", "node-manager"
- "ci", has forced low impact
- "docs", includes website changes, should have low impact
- "candi"
- "deckhouse-controller"
- "dhctl"
- "global-hooks"
- "go_lib"
- "helm_lib"
- "jq_lib"
- "shell_lib"
- "testing", has forced low impact
- "tools", has forced low impact

Find changed sections:

gh pr diff   $PULL_REQUEST_NUMBER   |
egrep "^([+]{3} b|[-]{3} a)/" |
cut -d/ -f2- |
sed 's#^ee/##' |
sed 's#^fe/##' |
sed 's#^modules/##' |
sed 's#[0-9][0-9][0-9]-##' |
egrep -v 'Makefile' |       # add file exclusion here
cut -d/ -f1 |
sort |
uniq

Find all possible sections (excluding ci):

node -e 'console.log(require("./.github/scripts/js/changelog-find-sections.js")().join("\n"))'
-->
    `;

  it('should parse markdown into tokens', () => {
    const tokens = parseMarkdown(testContent);

    // Verify that tokens are not empty
    expect(tokens).not.toHaveLength(0);

    expect(tokens[1].type).toEqual('heading');
    expect(tokens[1].text).toEqual('Description');
  });
});

describe('findSectionInMarkdown', () => {
  const mockTokens = [
    { type: 'heading', text: 'Changelog' },
    { type: 'code', lang: 'changes', text: 'Initial version' },
    { type: 'heading', text: 'Documentation' },
    { type: 'paragraph', text: 'Some docs' },
    { type: 'list', items: ['item1', 'item2'] }
  ];

  describe('default search (lang === "changes")', () => {
    it('should find section when exists', () => {
      const result = findSectionInMarkdown(mockTokens, 'Changelog');
      expect(result).toBe('Initial version');
    });

    it('should return null when header not found', () => {
      const result = findSectionInMarkdown(mockTokens, 'Non-existent Section');
      expect(result).toBeNull();
    });

    it('should return null when no matching token after header', () => {
      const modifiedTokens = mockTokens.filter(t => t.lang !== 'changes');
      const result = findSectionInMarkdown(modifiedTokens, 'Changelog');
      expect(result).toBeNull();
    });

    it('should stop searching at next header', () => {
      const tokens = [
        { type: 'heading', text: 'Changelog' },
        { type: 'paragraph', text: 'Should skip this' },
        { type: 'heading', text: 'Next Section' }, // stop here
        { type: 'code', lang: 'changes', text: 'Missed content' }
      ];
      
      const result = findSectionInMarkdown(tokens, 'Changelog', 'lang', 'changes');
      expect(result).toBeNull();
    });
  });

  describe('custom field and value search', () => {
    it('should find by type === "paragraph"', () => {
      const result = findSectionInMarkdown(
        mockTokens,
        'Documentation',
        'type',
        'paragraph'
      );
      expect(result).toBe('Some docs');
    });

    it('should handle non-existent field', () => {
      const result = findSectionInMarkdown(
        mockTokens,
        'Changelog',
        'unknownField',
        'value'
      );
      expect(result).toBeNull();
    });
  });

  describe('edge cases', () => {
    it('should handle case-insensitive header match', () => {
      const result = findSectionInMarkdown(mockTokens, 'changelog');
      expect(result).toBe('Initial version');
    });

    it('should return first matching token', () => {
      const tokens = [
        { type: 'heading', text: 'Section' },
        { type: 'code', lang: 'changes', text: 'First match' },
        { type: 'code', lang: 'changes', text: 'Second match' }
      ];
      
      const result = findSectionInMarkdown(tokens, 'Section');
      expect(result).toBe('First match');
    });

    it('should handle empty tokens array', () => {
      const result = findSectionInMarkdown([], 'Changelog');
      expect(result).toBeNull();
    });
  });
});
