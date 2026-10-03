# Writing documentation for an external module

This file describes what the docs-builder offers to authors of external module documentation:
which files are published, page parameters, shortcodes, render hooks and known pitfalls.
It is the contract between the docs-builder (`docs/site/backends/docs-builder`), this Hugo template and the module documentation.
The Claude Code plugin `deckhouse-platform-docs` reads this file from the `main` branch,
so keep it in sync with `layouts/` and the upload rules in the same merge request that changes them.

Editorial rules (style, terminology, EN/RU parity) are not part of the template.
They are published at <https://pp.flant.ru/llms.txt> and in the `deckhouse-writing` plugin.

## Published files

The docs-builder takes these files from the module repository:

| Path | Published as |
| --- | --- |
| `docs/**` | Documentation pages and their images |
| `crds/*.yaml`, `crds/*.yml`, `crds/*.json` | The `CR` page; only files directly in `crds/`, subdirectories are skipped |
| `crds/doc-<lang>-<file>.yaml` | Translations of the descriptions in `crds/<file>.yaml` on the `CR` page |
| `openapi/config-values.yaml` | The `CONFIGURATION` page |
| `openapi/doc-<lang>-config-values.yaml` | Translations of the `CONFIGURATION` page |
| `openapi/conversions/v<N>.yaml` | Conversions of module settings between schema versions |
| `module.yaml` | Lifecycle stage (`stage`), requirements (`requirements`) and the module description in the modules list (`descriptions`) |
| `oss.yaml` | Third-party components |

Not published:

- `docs/internal/`, `docs/internals/`, `docs/development/` and `docs/dev/`;
- subdirectories of `crds/`, which are the place for vendored or internal CRDs;
- other files of `openapi/`, such as `values.yaml`.

Other unpublished material inside `docs/` must be excluded from the module bundle (`excludePaths` in `.werf/bundle.yaml` of the module)
or moved under `docs/internal/`.

## Page files

- An English page is `<NAME>.md`, a Russian page is `<NAME>.ru.md`.
  The docs-builder also accepts `<NAME>_RU.md` and renames it to `.ru.md`, but new files use `.ru.md`.
- `README` is the module overview. In the module menu it is always the first page, regardless of `weight`.
- The docs-builder does not create the `CONFIGURATION` and `CR` pages.
  Create `docs/CONFIGURATION.md` and `docs/CR.md` (or `docs/CRD.md`) with front matter;
  the template appends the generated content after the page body:
  - on `CONFIGURATION`: the sections about enabling and configuring the module and changing its release channel,
    the `enablingNote` alert, the requirements from `module.yaml`, the conversions and the parameters.
    All of them are rendered only if `openapi/config-values.yaml` exists;
  - on `CR`: a note that the CRDs are not removed when the module is disabled, and the custom resources from `crds/`.
- A page is rendered as `<name>.html` in lowercase: `USER_GUIDE.md` becomes `user_guide.html`.

## Page parameters

| Parameter | Purpose |
| --- | --- |
| `title` | Page title |
| `description` | Short summary of the page, see [Page description](#page-description) |
| `weight` | Page order in the module menu |
| `menuTitle` | On README: the module name in the modules list, breadcrumbs and page navigation |
| `linkTitle` | Page title in the module menu, breadcrumbs and page navigation, except for the pages with fixed titles (see [Menu titles](#menu-titles)). On README: the module name in the modules list if `menuTitle` is not set |
| `search` | Comma-separated search keywords |
| `searchBoost` | Search ranking boost |
| `relatedLinks` | Related links at the end of the page, a list of `title` and `url`; also read from `params.relatedLinks`. See [Related links](#related-links) |
| `enablingNote` | On `CONFIGURATION`: Markdown text shown as an info alert in the section about enabling the module |

`moduleStatus` is deprecated: the stage comes from `stage` in `module.yaml`.

### Page description

The `description` parameter is used in the following places:

- the `description` meta tag of the page;
- the AI export of the documentation;
- on README: the module description in the modules list, if `module.yaml` has no `descriptions` for the page language.

Do not set `description` on `CONFIGURATION` and `CR` (`CRD`): if it is not set, the meta tag gets a description generated from the module name,
for example, "Deckhouse Platform — configuration parameters of the `<MODULE_NAME>` module".
Set `description` on every other page.
If it is not set there, the meta tag gets the first 199 characters of the Markdown source of the page body, without shortcode blocks.
If the resulting text is shorter than 20 characters, the meta tag gets the common site description,
"Kubernetes is flexibly and rapidly expanded by Deckhouse Platform modules."

### Menu titles

Pages with the following file names have fixed titles in the module menu, breadcrumbs and page navigation.
Their `title` and `linkTitle` are not used there.

| File | English title | Russian title |
| --- | --- | --- |
| `README` | Overview | Описание |
| `CONFIGURATION` | Configuration | Настройки |
| `CR`, `CRD` | Custom resources | Кастомные ресурсы |
| `USAGE` | Usage | Использование |
| `ADVANCED_USAGE` | Advanced usage | Расширенное использование |
| `EXAMPLES` | Examples | Примеры |
| `FAQ` | FAQ | FAQ |
| `SETUP` | Setup | Настройка |

Other pages are titled with `linkTitle`, or with `title` if `linkTitle` is not set.

### Related links

The `relatedLinks` list is rendered as the "Additional resources" section at the end of the page:

- no more than six links are shown;
- `title` can be omitted only for links to `/modules/...`: such a link is titled "Module" followed by the module name. Other links without `title` are skipped;
- the links open in a new tab.

## Shortcodes

Use only the shortcodes below, with their exact parameters. Liquid tags (`{% ... %}`) do not work.
Prefer the `{{< ... >}}` form: the shortcodes render the Markdown inside them themselves.

### Alert

```go-html-template
{{< alert level="warning" >}}
Text of the warning.
{{< /alert >}}
```

Levels: `info` (default), `warning`, `danger`. An unknown level falls back to `info`.

### Details

```go-html-template
{{< details summary="Example of the command output" >}}
Markdown content.
{{< /details >}}
```

The title is the named `summary` parameter. Without it, the block is titled "Details" on English pages and «Подробнее» on Russian pages.
The syntax matches the built-in Hugo `details` shortcode, but the template replaces it with its own:
only `summary` is supported; the other built-in parameters (`open`, `name`, `class`, `title`) are ignored.

### Tabs

```go-html-template
{{< tabs name="install_method" >}}
{{< tab name="Using the web interface" >}}
Markdown content.
{{< /tab >}}
{{< tab name="Using d8" >}}
Markdown content.
{{< /tab >}}
{{< /tabs >}}
```

`name` of `tabs` is optional: by default, every tab set on the page gets a unique identifier.
If you set `name`, make it unique on the page.
`name` of `tab` is the tab caption; it is required.

### Translate

`{{< translate <key> >}}` inserts a localised interface string of the template by its key.
The keys are listed in `i18n/en.yaml` and `i18n/ru.yaml` of the template.

## Render hooks and Markdown

- **Links.** Link to pages of the same module by their rendered names: `cr.html`, `user_guide.html#section`.
  Do not link to `.md` files.
  CR field anchors have the form `cr.html#<kind>-<version>-<field-path>` in lowercase,
  for example `cr.html#clusteralbinstance-v1alpha1-spec-inlet`.
  Module parameter anchors have the form `configuration.html#parameters-<parameter-path>` in lowercase, for example `configuration.html#parameters-https-mode`.
- **Link rewriting.** The site can rewrite, unwrap or filter links by rules in the site configuration.
  A link with the title `keep-link`, `[text](url "keep-link")`, is never rewritten.
- **Mermaid.** A fenced code block with the `mermaid` language renders as a diagram.
- **Code blocks.** Give every code block a language tag supported by Hugo (Chroma). Syntax is not guessed for untagged blocks.
- **Raw HTML** is allowed, but avoid it in pages.
- **Attributes.** Kramdown block attributes such as `{: .class}` do not work. Two kinds of attributes are supported:
  - a heading ID: `## Title {#custom-id}`. Use it to give the English and Russian headings the same anchor;
  - `{.nowrap-default}` after the language tag in the opening fence of a code block, for example `console {.nowrap-default}`:
    the block does not wrap long lines.

## Callouts in CRD and OpenAPI descriptions

Shortcodes are not rendered inside schema descriptions.
Write callouts there as a quote: `> **Warning.** ...` in English and `> **Внимание.** ...` in Russian.

## Local preview

Run from the root of the DP repository:

```shell
make -C docs/site external-module MODULE_PATH=<MODULE_REPO>
```

Open `http://localhost/products/kubernetes-platform/documentation/v1/` and go to `/modules/<module-name>/`.
Stop the preview with `make -C docs/site down`.
