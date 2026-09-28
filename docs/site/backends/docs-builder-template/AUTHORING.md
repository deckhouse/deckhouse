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
| `openapi/config-values.yaml` | The `CONFIGURATION` page |
| `openapi/doc-<lang>-config-values.yaml` | Translations of the `CONFIGURATION` page |
| `openapi/conversions/v<N>.yaml` | Conversions of module settings between schema versions |
| `module.yaml` | Module name, description, stage and editions |
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
- `README`, `CONFIGURATION` and `CR` are special pages: README is the module overview,
  the content of `CONFIGURATION` and `CR` is generated from `openapi/` and `crds/`.
- A page is rendered as `<name>.html` in lowercase: `USER_GUIDE.md` becomes `user_guide.html`.

## Page parameters

| Parameter | Purpose |
| --- | --- |
| `title` | Page title |
| `description` | Short summary for search and the modules list; generated for `CONFIGURATION` and `CR`, do not set it there |
| `weight` | Page order in the module menu |
| `menuTitle` | On README: the module name in the modules list, breadcrumbs and page navigation |
| `linkTitle` | Fallback for the module name in the modules list if `menuTitle` is not set |
| `search` | Comma-separated search keywords |
| `searchBoost` | Search ranking boost |
| `relatedLinks` | Related pages, a list of `title` and `url`; also read from `params.relatedLinks` |
| `enablingNote` | On `CONFIGURATION`: Markdown text shown as an info alert in the section about enabling the module |

`moduleStatus` is deprecated: the stage comes from `stage` in `module.yaml`.

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

The title is the named `summary` parameter. Without it, the block is titled "Details".
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

`name` of `tabs` is optional; set a unique one when a page has several tab sets.

### Translate

`{{< translate <key> >}}` inserts a localised interface string of the template by its key.

## Render hooks and Markdown

- **Links.** Link to pages of the same module by their rendered names: `cr.html`, `user_guide.html#section`.
  Do not link to `.md` files.
  CR field anchors have the form `cr.html#<kind>-<version>-<field-path>` in lowercase,
  for example `cr.html#clusteralbinstance-v1alpha1-spec-inlet`.
- **Link rewriting.** The site can rewrite, unwrap or filter links by rules in the site configuration.
  A link with the title `keep-link`, `[text](url "keep-link")`, is never rewritten.
- **Mermaid.** A fenced code block with the `mermaid` language renders as a diagram.
- **Code blocks.** Give every code block a language tag supported by Hugo (Chroma). Syntax is not guessed for untagged blocks.
- **Raw HTML** is allowed, but avoid it in pages.
- **Attributes.** Kramdown block attributes such as `{: .class}` do not work.

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
