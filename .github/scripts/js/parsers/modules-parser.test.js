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
const { STATIC_MODULES, findIn, isModule, walk } = require('./modules-parser');

jest.mock('fs');

function setupMockFileSystem(mockFs) {
  fs.readdirSync.mockImplementation((path) => {
    const entries = mockFs[path] || [];
    return entries.map(entry => ({
      ...entry,
      parentPath: path,
    }));
  });
}

describe('isModule', () => {
  it('returns true when module.yaml exists', () => {
    setupMockFileSystem({
      '/test': [
        { name: 'module.yaml', isFile: () => true, isDirectory: () => false }
      ]
    });
    expect(isModule('/test')).toBe(true);
  });

  it('returns true for Helm chart with images and hooks', () => {
    setupMockFileSystem({
      '/test': [
        { name: 'Chart.yaml', isFile: () => true },
        { name: 'images', isDirectory: () => true },
        { name: 'hooks', isDirectory: () => true }
      ]
    });
    expect(isModule('/test')).toBe(true);
  });

  it('returns true for Go module', () => {
    setupMockFileSystem({
      '/test': [
        { name: 'go.mod', isFile: () => true },
        { name: 'go.sum', isFile: () => true }
      ]
    });
    expect(isModule('/test')).toBe(true);
  });

  it('returns true for Helm with openapi', () => {
    setupMockFileSystem({
      '/test': [
        { name: 'Chart.yaml', isFile: () => true },
        { name: 'openapi', isDirectory: () => true }
      ]
    });
    expect(isModule('/test')).toBe(true);
  });

  it('returns false for non-module directory', () => {
    setupMockFileSystem({
      '/test': [
        { name: 'README.md', isFile: () => true }
      ]
    });
    expect(isModule('/test')).toBe(false);
  });
});

describe('walk', () => {
  it('finds modules recursively', () => {
    setupMockFileSystem({
      '/root': [
        { name: 'dir1', isDirectory: () => true }
      ],
      '/root/dir1': [
        { name: 'module-openapi', isDirectory: () => true }
      ],
      '/root/dir1/module-openapi': [
        { name: 'Chart.yaml', isFile: () => true },
        { name: 'openapi', isDirectory: () => true }
      ]
    });

    expect(walk('/root')).toEqual(['module-openapi']);
  });

  it('sanitizes directory names', () => {
    setupMockFileSystem({
      '/root': [
        { name: '001-module', isDirectory: () => true }
      ],
      '/root/001-module': [
        { name: 'module.yaml', isFile: () => true }
      ]
    });

    expect(walk('/root')).toEqual(['module']);
  });
});

describe('findIn', () => {
  it('includes static modules', () => {
    setupMockFileSystem({});
    expect(findIn()).toEqual(STATIC_MODULES.sort());
  });

  it('filters out "src" module', () => {
    setupMockFileSystem({
      '.': [
        { name: 'src', isDirectory: () => true }
      ],
      './src': [
        { name: 'go.mod', isFile: () => true },
        { name: 'go.sum', isFile: () => true }
      ]
    });

    const result = findIn('.');
    expect(result).not.toContain('src');
    expect(result).toEqual(STATIC_MODULES.sort());
  });

  it('deduplicates and sorts modules', () => {
    setupMockFileSystem({
      '.': [
        { name: 'ci', isDirectory: () => true }
      ],
      './ci': [
        { name: 'module.yaml', isFile: () => true }
      ]
    });

    const expected = [...STATIC_MODULES].sort();
    expect(findIn('.')).toEqual(expected);
  });

  it('combines found and static modules', () => {
    setupMockFileSystem({
      '.': [
        { name: 'custom', isDirectory: () => true }
      ],
      './custom': [
        { name: 'module.yaml', isFile: () => true }
      ]
    });

    const expected = [...STATIC_MODULES, 'custom'].sort();
    expect(findIn('.')).toEqual(expected);
  });
});