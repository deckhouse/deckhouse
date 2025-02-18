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

const { validatePullRequestChangelog } = require('./pr-changes-validator');

describe('validatePullRequestChangelog', () => {
  const allowedSections = ['valid_section', 'another_section'];

  test('valid single block', () => {
    const validEntry = `
section: valid_section
type: fix
summary: Valid summary
impact_level: default
`;
    expect(() => validatePullRequestChangelog(validEntry, allowedSections)).not.toThrow();
  });

  test('missing section', () => {
    const invalidEntry = `
type: fix
summary: Valid summary
impact_level: default
`;
    expect(() => validatePullRequestChangelog(invalidEntry, allowedSections))
      .toThrow("'section' is required and must be a non-empty string and allowed section in block 1");
  });

  test('invalid section', () => {
    const invalidEntry = `
section: invalid_section
type: fix
summary: Valid summary
impact_level: default
`;
    expect(() => validatePullRequestChangelog(invalidEntry, allowedSections))
      .toThrow("'section' is required and must be a non-empty string and allowed section in block 1");
  });

  test('invalid type', () => {
    const invalidEntry = `
section: valid_section
type: invalid_type
summary: Valid summary
impact_level: default
`;
    expect(() => validatePullRequestChangelog(invalidEntry, allowedSections))
      .toThrow("'type' must be one of type: fix, feature, chore. In block 1");
  });

  test('missing summary', () => {
    const invalidEntry = `
section: valid_section
type: fix
impact_level: default
`;
    expect(() => validatePullRequestChangelog(invalidEntry, allowedSections))
      .toThrow("'summary' is required and must be a non-empty string in block 1");
  });

  test('template summary', () => {
    const invalidEntry = `
section: valid_section
type: fix
summary: <ONE-LINE of what effectively changes for a user>
impact_level: default
`;
    expect(() => validatePullRequestChangelog(invalidEntry, allowedSections))
      .toThrow("'summary' is required and must be a non-empty string in block 1");
  });

  test('impact is template', () => {
    const invalidEntry = `
section: valid_section
type: fix
summary: Valid summary
impact: <what to expect for users, possibly MULTI-LINE>, required if impact_level is high ↓
impact_level: default
`;
    expect(() => validatePullRequestChangelog(invalidEntry, allowedSections))
      .toThrow("'impact' is required and must be a non-empty string in block 1");
  });

  test('invalid impact_level', () => {
    const invalidEntry = `
section: valid_section
type: fix
summary: Valid summary
impact_level: invalid
`;
    expect(() => validatePullRequestChangelog(invalidEntry, allowedSections))
      .toThrow("'impact_level' must be one of levels: default, high, low. In block 1");
  });

  test('multiple blocks with one invalid', () => {
    const validBlock = `
section: valid_section
type: fix
summary: Valid summary
impact_level: default
`;
    const invalidBlock = `
section: another_section
type: chore
summary: 
impact_level: high
`;
    const changelogEntries = `${validBlock}\n---\n${invalidBlock}`;
    expect(() => validatePullRequestChangelog(changelogEntries, allowedSections))
      .toThrow("'summary' is required and must be a non-empty string in block 2");
  });

  test('missing impact and impact_level', () => {
    const entry = `
section: valid_section
type: fix
summary: Valid summary
`;
    expect(() => validatePullRequestChangelog(entry, allowedSections))
    .toThrow("Cannot read properties of undefined (reading 'length')");
  });

  test('valid block with high impact_level without impact', () => {
    const entry = `
section: valid_section
type: fix
summary: Valid summary
impact_level: high
`;
    expect(() => validatePullRequestChangelog(entry, allowedSections)).not.toThrow();
  });
});