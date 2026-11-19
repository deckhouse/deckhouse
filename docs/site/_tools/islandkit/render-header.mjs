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
 *   node _tools/islandkit/render-header.mjs [--islandkit <checkout with dist/>] [--styles <dir with scss/>]
 *   node _tools/islandkit/render-header.mjs --mode module --menu <topnav.yml> --edition <EE|CE|...>
 *
 * Without --islandkit the package is taken from node_modules, which is what the
 * build does. With it, an unreleased build can be rendered locally; the header
 * styles are then taken from the same repository (packages/deckhouse-islands next
 * to packages/islandkit) unless --styles names another place.
 *
 * --mode module renders the header of the documentation built inside a cluster
 * (site.mode: module): a site with a menu of its own and one fragment per
 * language, written as _includes/_generated/header-module-<lang>.html.
 */

import { copyFileSync, cpSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
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

/*
 * The Russian header is a second design of the same island: a later breakpoint,
 * products and solutions of its own, the product drawn as a lockup. Everything
 * below is what header_ru.html and header-l2-menu_ru.liquid write out in
 * Liquid, moved into data.
 */

// Codes the Russian bar shows under another product - the same two lines that
// open header-l2-menu_ru.liquid.
const RU_ALIASES = { 'kubernetes-platform': 'deckhouse-platform', 'development-platform': 'development-portal' }

// Lockups the Russian bar has a picture for, by product code; the rest keep
// the mark and the title. The file names are the legacy template's.
const RU_LOCKUPS = {
  'deckhouse-platform': 'platform',
  stronghold: 'stronghold',
  'observability-platform': 'observability',
  code: 'code',
  'development-portal': 'development-platform',
}

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

/**
 * Solutions of the Russian design live in the Deckhouse Platform bar, after
 * Extensions, as on the main site. The menu feed sends them as a bare link to
 * /solutions/; the site opens it as a panel of solutions with descriptions,
 * which the feed does not carry, so the items come from header-solutions.json
 * and the entry's own title and address still come from the feed. A feed
 * without the entry - the copy of the data committed to the repo, which local
 * builds use - gets it inserted after Extensions, so the menu does not silently
 * lose it. The same rule the legacy bar followed.
 */
function withSolutions(items, solutions) {
  const children = solutions?.items ?? []
  if (children.length === 0) return items
  // The main site marks the entry with the CSS class menu-solutions, which the
  // menu feed carries in `classes`; the address is the legacy bar's own rule.
  const isSolutions = (item) =>
    (Array.isArray(item.classes) && item.classes.includes('menu-solutions')) || /^\/solutions\/?$/.test(item.url ?? '')
  if (items.some(isSolutions)) return items.map((item) => (isSolutions(item) ? { ...item, items: children } : item))
  const at = items.findIndex((item) => item.url === '/products/deckhouse-platform/extensions/')
  if (at === -1) return items
  return [...items.slice(0, at + 1), { title: solutions.title, url: '/solutions/', items: children }, ...items.slice(at + 1)]
}

function buildPayload({ lang, code, topnav, l2, products, solutions, labels }) {
  const ru = lang === 'ru'
  const groups = topnav.topnav?.[lang] ?? []
  // The menu feed may carry an entry named like the products label, which the
  // legacy template skipped by title: the products come from here.
  const topItems = groups
    .flatMap((group) => group.items ?? [])
    .filter((item) => item.title !== labels.products[lang])

  // "Products" is a menu entry like any other. Today it is written out by hand
  // in Liquid, three times over, once per layout that shows the header. The
  // Russian design gives its panel a grid of three columns.
  const menu = ru
    ? [{ title: labels.products[lang], url: '', columns: 3, items: products[lang] ?? [] }, ...topItems]
    : [{ title: labels.products[lang], url: '', items: products[lang] ?? [] }, ...topItems]

  // The row's own logo is a sprite symbol named after the product, the same
  // rule header-l2-menu.liquid uses to build it. The Russian bar draws the
  // product as a lockup where it has one.
  const branches = (l2[lang] ?? []).map((branch) => {
    const branchCode = /^\/products\/([^/]+)\//.exec(branch.url ?? '')
    if (!branchCode) return branch
    const lockup = ru ? RU_LOCKUPS[branchCode[1]] : undefined
    return {
      ...branch,
      ...(ru && branchCode[1] === 'deckhouse-platform' ? { items: withSolutions(branch.items ?? [], solutions[lang]) } : {}),
      icon: { sprite: SPRITE, symbol: branchCode[1] + '-icon' },
      // The Russian card keeps a place under the current item for the page's own
      // section tree; assets/js/islandkit-doc-tree.js fills it.
      ...(ru ? { currentSlot: true } : {}),
      ...(lockup ? { lockup: { src: '/images/header/logo-' + lockup + '.svg', alt: branch.title } } : {}),
    }
  })

  const productPath = '/products/' + ((ru && RU_ALIASES[code]) || code) + '/'

  // `rel: doc` is how this data has always marked the documentation entry, and
  // the island reads a flag rather than a rel value - so translate it here.
  //
  // The flag is a fallback the island uses only when the address matches
  // nothing. That is exactly the documentation case: the menu says
  // /documentation/v1/ while pages are served from /documentation/v1.73/. On a
  // page whose address does match something - /gs/, /features/ - that match
  // wins and the flag stands down.
  //
  // `section` is the other half of the same rule. _layouts/default.html marks
  // the entry whose link contains /gs/ or /guides/ on every page under that
  // segment. The Russian "Quick start" entry links to /gs/installer/, so a page
  // such as /gs/bm/step2.html is inside the section but not inside the link -
  // the entry declares the section root, and the island treats it as one.
  const sectionOf = (url) => {
    for (const segment of ['/gs/', '/guides/']) {
      const at = String(url ?? '').indexOf(segment)
      if (at !== -1 && at + segment.length < url.length) return url.slice(0, at + segment.length)
    }
    return undefined
  }
  const branchesWithCurrent = branches.map((branch) => ({
    ...branch,
    items: (branch.items ?? []).map((item) => {
      const section = sectionOf(item.url)
      return {
        ...item,
        ...(item.rel === 'doc' ? { current: true } : {}),
        ...(section ? { section } : {}),
      }
    }),
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
    // The Russian design switches at 1200 and draws the logo as a wordmark;
    // it has no social links in the sheet.
    ...(ru ? { breakpoint: 'xl' } : {}),
    logo: { title: 'Deckhouse', url: '/', icon: ru ? '/images/header/wordmark.svg' : '/images/logo-header.svg' },
    menu,
    productMenu: shownBranch,
    actions: [
      { key: 'docs', title: labels.documentation[lang], url: '/docs/', variant: 'secondary' },
      { key: 'contact', title: labels.contact_us_button[lang], event: 'request_callback', variant: 'primary' },
    ],
    // No language link: the English one is in the footer, the Russian design has none.
    social: ru
      ? []
      : [
          {
            title: 'GitHub',
            url: 'https://github.com/deckhouse/deckhouse',
            target: '_blank',
            icon: '/images/header/github.svg',
          },
        ],
  }
}

/**
 * The header of the documentation built inside a cluster.
 *
 * A different menu, not a different header: two or three flat entries from the
 * documentation module's topnav.yml, links prefixed with the page language,
 * some entries limited to editions. No products, no second row, no buttons -
 * the same as the legacy template printed in this mode. English keeps GitHub,
 * except in the edition built in one language; the language link is in the
 * footer, as on the public site.
 */
function buildModulePayload({ lang, topnav, edition }) {
  const ru = lang === 'ru'
  const oneLanguage = edition === 'CSE'
  const menu = (topnav.topnav?.[lang] ?? [])
    .flatMap((group) => group.items ?? [])
    .filter((item) => !Array.isArray(item.editions) || item.editions.length === 0 || item.editions.includes(edition))
    .map((item) =>
      item.external_url
        ? { title: item.title, url: item.external_url, target: '_blank' }
        : { title: item.title, url: '/' + lang + item.url },
    )

  return {
    locale: lang,
    ...(ru ? { breakpoint: 'xl' } : {}),
    logo: { title: 'Deckhouse', url: '/', icon: ru ? '/images/header/wordmark.svg' : '/images/logo-header.svg' },
    menu,
    productMenu: [],
    actions: [],
    social:
      ru || oneLanguage
        ? []
        : [{ title: 'GitHub', url: 'https://github.com/deckhouse/deckhouse', target: '_blank', icon: '/images/header/github.svg' }],
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

/**
 * The mount point, the markup inside it and the data both were built from.
 *
 * The Russian sheet is an accordion. That is an attribute rather than data: it
 * changes nothing in the server's markup - the sheet is built on first open.
 */
function wrap(markup, payload) {
  const sheet = payload.locale === 'ru' ? ' data-ik-sheet="collapsed"' : ''
  return [
    '{% raw %}',
    '<div data-ik-island="site-header" data-ik-props="' + PROPS_ID + '"' + sheet + '>',
    markup,
    '</div>',
    dataBlock(payload),
    '{% endraw %}',
    '',
  ].join('\n')
}

async function main() {
  const checkout = flag('--islandkit')
  const mode = flag('--mode') ?? 'site'
  if (mode !== 'site' && mode !== 'module') throw new Error('render-header: unknown --mode ' + mode)

  const packageRoot = checkout ? resolve(checkout) : join(SITE, 'node_modules', '@web', 'islandkit')
  const stylesRoot = flag('--styles')
    ? resolve(flag('--styles'))
    : checkout
      ? resolve(checkout, '..', 'deckhouse-islands')
      : join(SITE, 'node_modules', '@web', 'deckhouse-islands')
  const kit = await import(pathToFileURL(join(packageRoot, 'dist', 'index.js')).href)

  const island = kit.islands.find((one) => one.id === 'site-header')
  if (!island) throw new Error('the installed @web/islandkit ships no site-header island')

  const version = JSON.parse(readFileSync(join(packageRoot, 'package.json'), 'utf8')).version

  // A build taken from a working copy is not the release that carries the same
  // number, so it must not claim that address. The version lives in the path
  // precisely to be immutable; two different bundles under one number is the
  // one thing that breaks the promise.
  const assetVersion = checkout === null ? version : version + '-dev'

  const render = async (payload) => {
    const host = kit.resolveHostContext({ locale: payload.locale })
    const app = createSSRApp(island.component, island.parseProps(payload, host))
    kit.installContext(app, { host })
    return wrap(await renderToString(app), payload)
  }

  mkdirSync(OUT, { recursive: true })
  mkdirSync(DATA, { recursive: true })
  let products = []

  if (mode === 'module') {
    const edition = flag('--edition')
    const menuFile = flag('--menu')
    if (!edition || !menuFile) throw new Error('render-header: --mode module needs --edition and --menu')

    // Only this mode reads YAML: the module keeps its menu as YAML, and the
    // site build has no reason to install a parser it never uses.
    const { load } = await import('js-yaml')
    const topnav = load(readFileSync(resolve(menuFile), 'utf8'))

    for (const lang of LANGS) {
      const payload = buildModulePayload({ lang, topnav, edition })
      writeFileSync(join(OUT, 'header-module-' + lang + '.html'), await render(payload))
    }
    console.log('render-header  OK  - module header (' + edition + ') in _includes/_generated')
  } else {
    const topnav = readJson('topnav.json')
    const l2 = readJson('topnav-l2-products.json')
    const productsData = readJson('header-products.json')
    const solutions = readJson('header-solutions.json')
    const labels = readLabels(['products', 'documentation', 'contact_us_button'])

    let written = 0
    const missing = []
    products = productCodes(l2)
    for (const code of products) {
      for (const lang of LANGS) {
        const payload = buildPayload({ lang, code, topnav, l2, products: productsData, solutions, labels })
        if (payload.productMenu.length === 0) missing.push(code + '/' + lang)
        writeFileSync(join(OUT, 'header-' + code + '-' + lang + '.html'), await render(payload))
        written += 1
      }
    }
    console.log('render-header  OK  - ' + written + ' fragment(s) in _includes/_generated')
    if (missing.length > 0) {
      console.log('               no second row for: ' + missing.join(', ') + ' (no branch in topnav-l2-products)')
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

  // Older versions go here and now. Nothing links to them any more - the address
  // comes from islandkit_header.json - and left behind they pile up in the tree
  // at 188 KB a release with nobody to remove them.
  const stale = readdirSync(assetsRoot).filter((name) => name !== assetVersion)
  for (const name of stale) rmSync(join(assetsRoot, name), { recursive: true, force: true })

  // Two things the templates read from here: which products have a fragment, so
  // an unknown one falls back instead of failing the build, and which asset
  // directory to link - written once here rather than typed into head-site.html
  // and forgotten on the next release.
  writeFileSync(join(DATA, 'islandkit_header.json'), JSON.stringify({ products, version, assetVersion }, null, 2) + '\n')

  // The header's look: Sass from @web/deckhouse-islands, which site.scss pulls
  // in through _islandkit-header.scss. It comes from the same place as the
  // bundle for the same reason - one pin per release, nothing kept in the tree.
  const stylesOut = join(SITE, '_sass', 'deckhouse-islands')
  rmSync(stylesOut, { recursive: true, force: true })
  cpSync(join(stylesRoot, 'scss'), stylesOut, { recursive: true })
  const stylesVersion = JSON.parse(readFileSync(join(stylesRoot, 'package.json'), 'utf8')).version

  console.log('               bundle v' + version + ' in assets/islandkit/' + assetVersion)
  console.log('               styles v' + stylesVersion + ' in _sass/deckhouse-islands')
}

await main()
