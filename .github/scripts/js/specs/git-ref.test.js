const {
  parseGitRef
} = require('../git-ref')

test('valid release tags', () => {
  const refs = [
    {
      ref: 'refs/tags/v1.0.0',
      tagVersion: 'v1.0.0',
    },
    {
      ref: 'refs/tags/v1.0.0',
      tagVersion: 'v1.0.0',
    },
  ]
  refs.forEach(test => {
    const gitInfo = parseGitRef(test.ref)
    expect(gitInfo).not.toBeNull()
    expect(gitInfo.ref).toEqual(test.ref)
    expect(gitInfo.description).not.toEqual('')
    expect(gitInfo.isTag).toBeTruthy()
    expect(gitInfo.tagVersion).toEqual(test.tagVersion)
  })
})

test('valid developer tags', () => {
  const refs = [
    {
      ref: 'refs/tags/dev-v1.0.0',
    },
    {
      ref: 'refs/tags/pr-v1.0.0-12',
    }
  ]
  refs.forEach(test => {
    const gitInfo = parseGitRef(test.ref)
    expect(gitInfo).not.toBeNull()
    expect(gitInfo.ref).toEqual(test.ref)
    expect(gitInfo.description).not.toEqual('')
    expect(gitInfo.isTag).toBeTruthy()
    expect(gitInfo.isDeveloperTag).toBeTruthy()
  })
})

test('valid release branches', () => {
  const refs = [
    {
      ref: 'refs/heads/release-1.01',
      branchMajorMinor: 'v1.01',
    },
  ]
  refs.forEach(test => {
    const gitInfo = parseGitRef(test.ref)
    expect(gitInfo).not.toBeNull()
    expect(gitInfo.ref).toEqual(test.ref)
    expect(gitInfo.description).not.toEqual('')
    expect(gitInfo.isReleaseBranch).toBeTruthy()
    expect(gitInfo.branchMajorMinor).toEqual(test.branchMajorMinor)
  })
})
