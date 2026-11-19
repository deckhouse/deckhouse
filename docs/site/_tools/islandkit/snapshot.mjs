/**
 * Writes the islandKit snapshot: what a build without a registry token uses
 * instead of a fresh render (use-snapshot.sh, DOCS_ISLANDKIT_SNAPSHOT=1).
 *
 * The snapshot holds only what the site publishes anyway - the rendered
 * fragments, their index, the bundle and compiled CSS. No source of the
 * private packages goes into the repository: the styles package is Sass, so
 * its part of the header styles is compiled here, and the snapshot carries the
 * result together with empty mixins of the same names for
 * _sass/components/_islandkit-package.scss to include.
 *
 * Usage: node _tools/islandkit/snapshot.mjs
 *
 * The @web packages come from the private registry. With FOX_NPM_TOKEN (or
 * CI_JOB_TOKEN) in the environment the script points npm at it, the way the
 * header stage does; without one npm has to find the packages on its own,
 * through ~/.npmrc. Nothing is installed into the working tree: the render
 * runs in a temporary copy of the inputs the header stage takes.
 */

import { execFileSync } from 'node:child_process'
import { cpSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const HERE = fileURLToPath(new URL('.', import.meta.url))
const SITE = resolve(HERE, '..', '..')
const REPO = resolve(SITE, '..', '..')
const SNAPSHOT = join(HERE, 'snapshot')
const MODULE_MENU = join(REPO, 'modules', '810-documentation', 'images', 'web', 'documentation', '_data', 'topnav.yml')
const PACKAGE_STYLES = join('_sass', 'components', '_islandkit-package.scss')

// The inputs of the header stage (docs/site/werf-islandkit.inc.yaml), with the
// menu from the repository, plus the Sass that includes the styles package.
const INPUTS = [
  join('_tools', 'islandkit', 'render-header.mjs'),
  join('_data', 'header-products.json'),
  join('_data', 'header-solutions.json'),
  join('_data', 'i18n.yaml'),
  join('_data', 'topnav.json'),
  join('_data', 'topnav-l2-products.json'),
  '_config.yml',
  join('pages', 'includes'),
  PACKAGE_STYLES,
]

// The registry of the @web scope, as in .werf/defines/npm-private-registry.tmpl.
// The token covers all of /api/v4/: npm reads the metadata from the instance
// endpoint, and GitLab serves the tarball from the project's one.
const REGISTRY_HOST = 'fox.flant.com/api/v4/'

const pins = JSON.parse(readFileSync(join(HERE, 'versions.json'), 'utf8'))
const work = mkdtempSync(join(tmpdir(), 'islandkit-snapshot-'))

try {
  for (const input of INPUTS) cpSync(join(SITE, input), join(work, input), { recursive: true })
  writeFileSync(join(work, 'package.json'), JSON.stringify({ private: true }) + '\n')

  // The token goes into a config file inside the temporary directory, removed
  // with it, and never onto a command line.
  const npmEnv = { ...process.env }
  const token = process.env.FOX_NPM_TOKEN || process.env.CI_JOB_TOKEN
  if (token) {
    const npmrc = join(work, 'npmrc')
    writeFileSync(
      npmrc,
      '@web:registry=https://' + REGISTRY_HOST + 'packages/npm/\n//' + REGISTRY_HOST + ':_authToken=' + token + '\n',
      { mode: 0o600 },
    )
    npmEnv.NPM_CONFIG_USERCONFIG = npmrc
  } else {
    console.log('snapshot       no FOX_NPM_TOKEN: npm takes the registry of @web from its own configuration')
  }

  // `--ignore-scripts` for the reason the stage has it: no lifecycle scripts of
  // other people's packages.
  execFileSync(
    'npm',
    [
      'install', '--no-audit', '--no-fund', '--ignore-scripts',
      '@web/islandkit@' + pins.islandkit,
      '@web/deckhouse-islands@' + pins.islandkit,
      'vue@' + pins.vue,
      '@vue/server-renderer@' + pins.vue,
      'js-yaml@' + pins.jsYaml,
      'sass@' + pins.sass,
    ],
    { cwd: work, stdio: 'inherit', env: npmEnv },
  )

  const require = createRequire(join(work, 'package.json'))
  const render = (...args) =>
    execFileSync(process.execPath, [join(work, INPUTS[0]), ...args], { cwd: work, stdio: 'inherit' })
  const generated = join(work, '_includes', '_generated')
  const index = join(work, '_data', 'islandkit_header.json')

  // Assembled aside and swapped in at the end: a failed run leaves the
  // previous snapshot as it was.
  const out = join(work, 'snapshot')

  render()
  cpSync(generated, join(out, 'site', '_generated'), { recursive: true })
  cpSync(index, join(out, 'site', 'islandkit_header.json'))

  // One directory per edition: the stage takes the one of the build by name,
  // and the rule of what an edition shows stays in render-header.mjs alone.
  const editions = require('js-yaml')
    .load(readFileSync(join(REPO, 'editions.yaml'), 'utf8'))
    .editions.map((edition) => edition.name)
  for (const edition of editions) {
    rmSync(generated, { recursive: true, force: true })
    render('--mode', 'module', '--menu', MODULE_MENU, '--edition', edition)
    cpSync(generated, join(out, 'module', edition), { recursive: true })
  }
  cpSync(index, join(out, 'module', 'islandkit_header.json'))

  cpSync(join(work, 'assets', 'islandkit'), join(out, 'assets'), { recursive: true })

  const source = readFileSync(join(work, PACKAGE_STYLES), 'utf8')
  const mixins = [...new Set([...source.matchAll(/@include\s+islands\.([\w-]+)/g)].map((match) => match[1]))]
  if (mixins.length === 0) throw new Error('snapshot: ' + PACKAGE_STYLES + ' includes no mixin of the styles package')
  const { css } = require('sass').compile(join(work, PACKAGE_STYLES), {
    loadPaths: [join(work, '_sass')],
    style: 'compressed',
  })
  mkdirSync(join(out, 'styles'), { recursive: true })
  writeFileSync(join(out, 'styles', 'compiled.css'), css + '\n')
  writeFileSync(
    join(out, 'styles', '_index.scss'),
    [
      '// Written by _tools/islandkit/snapshot.mjs, do not edit.',
      '// Stands in for the Sass of @web/deckhouse-islands in a build from the',
      '// snapshot: the package part of the header styles is already compiled, and',
      '// the mixins that produce it in a normal build are empty here.',
      '@use "sass:meta";',
      '@use "compiled";',
      '',
      // An argument list takes named arguments only once its keywords are read.
      ...mixins.map((name) => '@mixin ' + name + '($args...) { $_: meta.keywords($args); }'),
      '',
    ].join('\n'),
  )

  rmSync(SNAPSHOT, { recursive: true, force: true })
  cpSync(out, SNAPSHOT, { recursive: true })

  console.log('snapshot       OK  - ' + editions.length + ' edition(s), version ' + pins.islandkit + ' in _tools/islandkit/snapshot')
} finally {
  rmSync(work, { recursive: true, force: true })
}
