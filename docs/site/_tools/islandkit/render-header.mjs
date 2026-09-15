/**
 * Prints the header fragments this site serves over SSI.
 *
 * The header is a content island: its markup has to be in the server's answer,
 * not built by script after load. The library ships a server renderer for PHP
 * hosts; this build has Ruby, so the markup here is printed by the same Vue
 * component that runs in the browser, through Vue's own server renderer. That
 * leaves no third description of the structure to keep in step - parity holds
 * by construction rather than by a test.
 *
 * One fragment per product and language, which is how this site already works:
 * the header is built once per product, published as a static include, and
 * stitched into pages it knows nothing about by nginx.
 *
 * Usage:
 *   node _tools/islandkit/render-header.mjs [--islandkit <checkout with dist/>]
 *
 * Without the flag the package is taken from node_modules, which is what the
 * build does. With it, an unreleased build can be rendered locally.
 */

import { copyFileSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'

const HERE = fileURLToPath(new URL('.', import.meta.url))

// `--site` exists so an unreleased library can be rendered from its own
// checkout, where vue and its server renderer are already installed. The
// build never passes it: there the script sits inside the site it renders.
const siteFlag = process.argv.indexOf('--site')
const SITE = siteFlag === -1 ? resolve(HERE, '..', '..') : resolve(process.argv[siteFlag + 1])
const DATA = join(SITE, '_data')
const OUT = join(SITE, '_includes', '_generated')
const LANGS = ['en', 'ru']
const PROPS_ID = 'ik-header-data'
const SPRITE = '/images/sprite.svg?v2'

const args = process.argv.slice(2)
const flag = (name) => {
  const at = args.indexOf(name)
  return at === -1 ? null : args[at + 1]
}

const readJson = (file) => JSON.parse(readFileSync(join(DATA, file), 'utf8'))

/**
 * Reads the two-level keys the header needs out of the site's i18n file.
 *
 * A YAML parser would be a dependency taken on for three strings. The shape
 * being read is a key followed by indented `en:` and `ru:`; anything else
 * throws here rather than rendering an empty label into every page.
 */
function readLabels(keys) {
  const lines = readFileSync(join(DATA, 'i18n.yaml'), 'utf8').split(/\r?\n/)
  const out = {}

  for (const key of keys) {
    const at = lines.findIndex((line) => line.trimEnd() === '  ' + key + ':')
    if (at === -1) throw new Error('i18n.yaml has no key ' + key)

    const found = {}
    for (const line of lines.slice(at + 1, at + 4)) {
      const match = /^ {4}(en|ru): *"?(.*?)"? *$/.exec(line)
      if (match) found[match[1]] = match[2]
    }
    for (const lang of LANGS) {
      if (!found[lang]) throw new Error('i18n.yaml key ' + key + ' has no ' + lang)
    }
    out[key] = found
  }

  return out
}

/**
 * Every product the site can ask for.
 *
 * Three sources, because the code arrives three ways: the stubs that publish
 * the SSI fragments, the path defaults in _config.yml, and the branches of the
 * second row. Missing one of them fails the build, not the page - which is how
 * this was found.
 */
function productCodes(l2) {
  const codes = new Set(['kubernetes-platform']) // the default in header-l2-menu.liquid

  const dir = join(SITE, 'pages', 'includes')
  for (const name of readdirSync(dir)) {
    if (!name.startsWith('header')) continue
    const found = /^product_code: *(\S+) *$/m.exec(readFileSync(join(dir, name), 'utf8'))
    if (found) codes.add(found[1])
  }

  const config = readFileSync(join(SITE, '_config.yml'), 'utf8')
  for (const found of config.matchAll(/^ *product_code: *(\S+) *$/gm)) codes.add(found[1])

  for (const lang of LANGS) {
    for (const branch of l2[lang] ?? []) {
      const found = /^\/products\/([^/]+)\/$/.exec(branch.url ?? '')
      if (found) codes.add(found[1])
    }
  }

  return [...codes].sort()
}

function buildPayload({ lang, code, topnav, l2, products, labels }) {
  const groups = topnav.topnav?.[lang] ?? []
  const topItems = groups.flatMap((group) => group.items ?? [])

  // "Products" is a menu entry like any other. Today it is written out by hand
  // in Liquid, three times over, once per layout that shows the header.
  const menu = [{ title: labels.products[lang], url: '', items: products[lang] ?? [] }, ...topItems]

  // The row's own logo is a sprite symbol named after the product, the same
  // rule header-l2-menu.liquid uses to build it.
  const branches = (l2[lang] ?? []).map((branch) => {
    const branchCode = /^\/products\/([^/]+)\//.exec(branch.url ?? '')
    return branchCode
      ? { ...branch, icon: { sprite: SPRITE, symbol: branchCode[1] + '-icon' } }
      : branch
  })

  const productPath = '/products/' + code + '/'

  // `rel: doc` is how this data has always marked the documentation entry, and
  // the island reads a flag rather than a rel value - so translate it here.
  //
  // The flag is a fallback the island uses only when the address matches
  // nothing. That is exactly the documentation case: the menu says
  // /documentation/v1/ while pages are served from /documentation/v1.73/. On a
  // page whose address does match something - /gs/, /features/ - that match
  // wins and the flag stands down.
  const branchesWithCurrent = branches.map((branch) => ({
    ...branch,
    items: (branch.items ?? []).map((item) => (item.rel === 'doc' ? { ...item, current: true } : item)),
  }))

  // Only the branch this fragment actually shows. The island picks one branch
  // out of `productMenu` and prints that one; shipping the other nine put 5 KB
  // of JSON - 48% of the fragment - into every page of every product, where
  // nothing could ever read it.
  const shownBranch = branchesWithCurrent.filter((branch) => branch.url === productPath)

  // Deliberately no `currentPath`: the fragment is served on pages it knows
  // nothing about, and a baked-in address would be wrong on most of them AND
  // would stop the island from reading the real one from the browser.
  return {
    locale: lang,
    productPath,
    logo: { title: 'Deckhouse', url: '/', icon: '/images/logo-header.svg' },
    menu,
    productMenu: shownBranch,
    actions: [
      { key: 'docs', title: labels.documentation[lang], url: '/docs/', variant: 'secondary' },
      { key: 'contact', title: labels.contact_us_button[lang], event: 'request_callback', variant: 'primary' },
    ],
    social: [
      {
        title: 'GitHub',
        url: 'https://github.com/deckhouse/deckhouse',
        target: '_blank',
        icon: '/images/social-github.svg',
      },
    ],
  }
}

/**
 * The data block, safe to sit inside a `<script>` element.
 *
 * `JSON.stringify` escapes neither `<` nor `/`, so a single `</script>` in a
 * menu title - a CMS field anyone can edit - would close the element early and
 * spill the rest of the payload into the page as markup. Escaping the angle
 * bracket as `<` keeps the JSON identical to a parser and inert to an HTML
 * one. U+2028/U+2029 go too: legal in JSON, fatal in a script body.
 */
function dataBlock(payload) {
  const json = JSON.stringify(payload)
    .replace(/</g, '\\u003c')
    .replace(/\u2028/g, '\\u2028')
    .replace(/\u2029/g, '\\u2029')

  return '<script type="application/json" id="' + PROPS_ID + '">' + json + '</scr' + 'ipt>'
}

/** The mount point, the markup inside it and the data both were built from. */
function wrap(markup, payload) {
  return [
    '{% raw %}',
    '<div data-ik-island="site-header" data-ik-props="' + PROPS_ID + '">',
    markup,
    '</div>',
    dataBlock(payload),
    '{% endraw %}',
    '',
  ].join('\n')
}

async function main() {
  const checkout = flag('--islandkit')
  const packageRoot = checkout ? resolve(checkout) : join(SITE, 'node_modules', 'islandkit')
  const kit = await import(pathToFileURL(join(packageRoot, 'dist', 'index.js')).href)

  const island = kit.islands.find((one) => one.id === 'site-header')
  if (!island) throw new Error('the installed islandkit ships no site-header island')

  const topnav = readJson('topnav.json')
  const l2 = readJson('topnav-l2-products.json')
  const products = readJson('header-products.json')
  const labels = readLabels(['products', 'documentation', 'contact_us_button'])

  const version = JSON.parse(readFileSync(join(packageRoot, 'package.json'), 'utf8')).version

  // A build taken from a working copy is not the release that carries the same
  // number, so it must not claim that address. The version lives in the path
  // precisely to be immutable; two different bundles under one number is the
  // one thing that breaks the promise.
  const assetVersion = checkout === null ? version : version + '-dev'

  mkdirSync(OUT, { recursive: true })
  let written = 0
  const missing = []

  for (const code of productCodes(l2)) {
    for (const lang of LANGS) {
      const payload = buildPayload({ lang, code, topnav, l2, products, labels })
      if (payload.productMenu.length === 0) {
        missing.push(code + '/' + lang)
      }

      const host = kit.resolveHostContext({ locale: lang })
      const props = island.parseProps(payload, host)

      const app = createSSRApp(island.component, props)
      kit.installContext(app, { host })

      writeFileSync(join(OUT, 'header-' + code + '-' + lang + '.html'), wrap(await renderToString(app), payload))
      written += 1
    }
  }

  // The bundle travels with the fragments: one registry, one version, and no
  // way for the markup and the script that replaces it to come from different
  // releases.
  const assetsRoot = join(SITE, 'assets', 'islandkit')
  const assets = join(assetsRoot, assetVersion)
  mkdirSync(assets, { recursive: true })
  for (const file of ['standalone.js', 'standalone.css']) {
    copyFileSync(join(packageRoot, 'dist', file), join(assets, file))
  }

  // Прежние версии убираются здесь же. Ничего на них уже не ссылается - адрес
  // берётся из islandkit_header.json, - а оставленные они копятся в дереве по
  // 188 КБ на выпуск, и удалить их потом некому.
  const stale = readdirSync(assetsRoot).filter((name) => name !== assetVersion)
  for (const name of stale) rmSync(join(assetsRoot, name), { recursive: true, force: true })

  // Two things the templates read from here: which products have a fragment, so
  // an unknown one falls back instead of failing the build, and which asset
  // directory to link - written once here rather than typed into head-site.html
  // and forgotten on the next release.
  writeFileSync(
    join(DATA, 'islandkit_header.json'),
    JSON.stringify({ products: productCodes(l2), version, assetVersion }, null, 2) + '\n',
  )

  console.log('render-header  OK  - ' + written + ' fragment(s) in _includes/_generated')
  console.log('               bundle v' + version + ' in assets/islandkit/' + assetVersion)
  if (missing.length > 0) {
    console.log('               no second row for: ' + missing.join(', ') + ' (no branch in topnav-l2-products)')
  }
}

await main()
