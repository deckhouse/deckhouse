# Generates a self-contained, single-file "print" HTML of the Deckhouse Code
# documentation for each language (en, ru).
#
# It mirrors the Hugo reference (hugo-web-product-module: layouts/documentation/
# list.print.html + baseof.print.html + static/css/print.css): a cover, a table
# of contents and every `code` documentation page concatenated in sidebar order
# (docs/site/_data/sidebars/code.yml).
#
# Output: <lang>/print/code/documentation/index.html
#
# The page bodies are taken from each page's already-rendered `content`
# (collected in a :site, :post_render hook), so custom Liquid tags (alert,
# offtopic, include), code highlighting and includes are all resolved. The CSS
# is inlined and referenced images (/images/...) are embedded as data URIs, so
# the result is a single portable file ready for conversion to other formats.

require "base64"
require "uri"

module CodePrint
  LANGS = %w[en ru].freeze

  DOC_TITLE = {
    "en" => "Deckhouse Code documentation",
    "ru" => "Документация Deckhouse Code",
  }.freeze

  TOC_TITLE = {
    "en" => "Table of contents",
    "ru" => "Содержание",
  }.freeze

  GEN_PREFIX = {
    "en" => "Generated:",
    "ru" => "Дата формирования:",
  }.freeze

  RU_MONTHS = %w[
    января февраля марта апреля мая июня
    июля августа сентября октября ноября декабря
  ].freeze

  MIME_BY_EXT = {
    ".png" => "image/png", ".jpg" => "image/jpeg", ".jpeg" => "image/jpeg",
    ".gif" => "image/gif", ".svg" => "image/svg+xml", ".webp" => "image/webp",
    ".ico" => "image/x-icon",
  }.freeze

  PRINT_CSS = <<~'CSS'
    @page {
      size: A4;
      margin: 22mm 15mm 20mm 15mm;
      @bottom-right { content: counter(page); font-size: 10pt; color: #57606a; }
    }
    @page cover { margin: 0; @bottom-right { content: none; } }
    @page :first { @top-left { content: none; } }

    * { box-sizing: border-box; }
    html, body {
      margin: 0; padding: 0;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "Helvetica Neue", Arial, sans-serif;
      font-size: 11pt; line-height: 1.5; color: #1f2328;
    }

    /* ---------- Cover ---------- */
    .pdf-cover {
      page: cover;
      break-after: page; page-break-after: always;
      position: relative; width: 210mm; height: 297mm;
      margin: 0; padding: 0; text-align: center;
    }
    .pdf-cover > div:empty { display: none; }
    .pdf-cover__title {
      position: absolute; top: 50%; left: 20mm; right: 20mm;
      transform: translateY(-50%); margin: 0;
      font-size: 32pt; font-weight: 700; line-height: 1.2;
    }
    .pdf-cover__date {
      position: absolute; left: 20mm; right: 20mm; bottom: 25mm;
      margin: 0; font-size: 12pt; color: #57606a;
    }

    /* ---------- TOC ---------- */
    .pdf-toc { break-after: page; page-break-after: always; }
    .pdf-toc__title { font-size: 24pt; margin: 0 0 16px 0; }
    .pdf-toc__list { list-style: none; padding: 0; margin: 0; }
    .pdf-toc__item { margin: 4px 0; }
    .pdf-toc__item a { color: #1f2328; text-decoration: none; display: block; }
    .pdf-toc__item--level-0 > a { font-weight: 600; font-size: 12pt; }
    .pdf-toc__item--level-1 > a { font-size: 11pt; }
    .pdf-toc__item--level-2 > a { font-size: 10pt; color: #57606a; }
    .pdf-toc__item a[href^="#"]::after {
      content: leader('.') " " target-counter(attr(href url), page);
      color: #57606a; font-variant-numeric: tabular-nums; font-size: 10pt;
    }

    /* ---------- Pages ---------- */
    .pdf-page { margin-top: 16pt; }
    .pdf-page__title {
      break-after: avoid; page-break-after: avoid;
      margin: 0 0 12pt 0; line-height: 1.2;
    }
    .pdf-page__title--level-0 { font-size: 24pt; font-weight: 700; }
    .pdf-page__title--level-1 { font-size: 18pt; font-weight: 700; }
    .pdf-page__title--level-2 { font-size: 14pt; font-weight: 600; }
    .pdf-page--level-0 { break-before: page; page-break-before: always; }

    .pdf-page h2, .pdf-page h3, .pdf-page h4, .pdf-page h5, .pdf-page h6 {
      font-weight: 600; break-after: avoid; page-break-after: avoid; margin-bottom: 6pt;
    }
    .pdf-page h4, .pdf-page h5, .pdf-page h6 { color: #57606a; }
    .pdf-page--level-0 h2 { font-size: 18pt; margin-top: 16pt; }
    .pdf-page--level-0 h3 { font-size: 14pt; margin-top: 14pt; }
    .pdf-page--level-0 h4 { font-size: 12pt; margin-top: 12pt; }
    .pdf-page--level-1 h2 { font-size: 14pt; margin-top: 14pt; }
    .pdf-page--level-1 h3 { font-size: 12pt; margin-top: 12pt; }
    .pdf-page--level-1 h4 { font-size: 11pt; margin-top: 10pt; }

    p, ul, ol { margin: 0 0 8pt 0; }
    a { color: #0969da; }

    pre, code {
      font-family: "SF Mono", Menlo, Consolas, "Liberation Mono", monospace;
      font-size: 9.5pt;
    }
    pre, .highlight, .highlighter-rouge {
      background: #f6f8fa; border: 1px solid #d0d7de; border-radius: 4px;
    }
    pre {
      padding: 10px 12px; overflow: hidden;
      white-space: pre-wrap; word-wrap: break-word;
      break-inside: avoid; page-break-inside: avoid;
    }
    .highlight { padding: 0; }
    .highlight pre, pre.highlight { border: 0; margin: 0; }
    code { background: #f6f8fa; padding: 1px 4px; border-radius: 3px; }
    pre code, .highlight code { background: transparent; padding: 0; border: 0; }

    table {
      border-collapse: collapse; width: 100%; margin: 8pt 0;
      break-inside: avoid; page-break-inside: avoid;
    }
    th, td { border: 1px solid #d0d7de; padding: 6px 10px; text-align: left; vertical-align: top; }
    th { background: #f6f8fa; font-weight: 600; }

    img { max-width: 100%; height: auto; }
    figure { break-inside: avoid; page-break-inside: avoid; margin: 8pt 0; text-align: center; }

    blockquote {
      border-left: 4px solid #d0d7de; padding: 4px 12px; margin: 8pt 0; color: #57606a;
    }

    /* ---------- Alerts (Jekyll _plugins/alert.rb markup) ---------- */
    .alert__wrap {
      border-left: 4px solid #0969da; padding: 12px 16px; margin: 12px 0;
      background: #ddf4ff; border-radius: 4px;
      break-inside: avoid; page-break-inside: avoid;
    }
    .alert__wrap .alert__icon { display: none; }
    .alert__wrap > div { margin: 0; }
    .alert__wrap p { margin: 0 0 6pt 0; }
    .alert__wrap p:last-child { margin-bottom: 0; }
    .info.alert__wrap    { border-color: #0969da; background: #ddf4ff; }
    .warning.alert__wrap { border-color: #bf8700; background: #fff8c5; }
    .danger.alert__wrap  { border-color: #cf222e; background: #ffebe9; }
    .alert__wrap.hide { display: block; }

    /* ---------- Details / offtopic (always expanded in print) ---------- */
    .details { margin: 8pt 0; border: 1px solid #d0d7de; border-radius: 4px; }
    .details__lnk { margin: 0; padding: 6px 12px; background: #f6f8fa; border-bottom: 1px solid #d0d7de; }
    .details__summary { font-weight: 600; text-decoration: none; color: #1f2328; pointer-events: none; }
    .details__content { display: block !important; padding: 12px; }
    .details__content .expand { display: block !important; }
  CSS

  module_function

  def generated_date(lang)
    t = Time.now
    if lang == "ru"
      "#{t.day} #{RU_MONTHS[t.month - 1]} #{t.year} г."
    else
      t.strftime("%B %-d, %Y")
    end
  end

  # Normalize any doc URL to a language-agnostic key:
  #   /en/code/documentation/admin/           -> code/documentation/admin
  #   /code/documentation/admin/x.html        -> code/documentation/admin/x.html
  def norm(url)
    s = url.to_s.strip
    s = s.sub(%r{\A/}, "")
    s = s.sub(%r{\A(en|ru)/}, "")
    s = s.sub(%r{/index\.html\z}, "")
    s = s.sub(%r{/\z}, "")
    s
  end

  def anchor_for(url)
    base = norm(url).gsub(/[^A-Za-z0-9]+/, "-").gsub(/\A-+|-+\z/, "")
    base = "root" if base.empty?
    "cp-#{base}".downcase
  end

  def esc(str)
    str.to_s.gsub("&", "&amp;").gsub("<", "&lt;").gsub(">", "&gt;")
  end

  # Flatten the sidebar into an ordered list of nodes:
  #   { title:, level:, anchor:, url: (nil for group headings) }
  def flatten_sidebar(entries, lang)
    nodes = []
    group_idx = 0
    entries.each do |entry|
      title = entry.dig("title", lang) || entry.dig("title", "en")
      if entry["folders"]
        group_idx += 1
        nodes << { title: title, level: 0, anchor: "cp-group-#{group_idx}", url: nil }
        entry["folders"].each do |folder|
          ftitle = folder.dig("title", lang) || folder.dig("title", "en")
          nodes << { title: ftitle, level: 1, anchor: anchor_for(folder["url"]), url: folder["url"] }
        end
      else
        nodes << { title: title, level: 0, anchor: anchor_for(entry["url"]), url: entry["url"] }
      end
    end
    nodes
  end

  # Index rendered `code` pages by normalized URL, per language.
  def build_index(site)
    index = { "en" => {}, "ru" => {} }
    site.pages.each do |page|
      next unless page.data["product_code"] == "code"
      next if page.data["is_code_print"]

      lang = page.data["lang"] || "en"
      next unless index.key?(lang)

      index[lang][norm(page.url)] = page
    end
    index
  end

  # Rewrite intra-doc links (that resolve to a known code page) into #anchors.
  def rewrite_links(html, base_url, anchor_map)
    html.gsub(/href=("|')(.*?)\1/m) do
      quote = Regexp.last_match(1)
      href = Regexp.last_match(2)
      anchor = href_to_anchor(href, base_url, anchor_map)
      anchor ? %(href=#{quote}#{anchor}#{quote}) : Regexp.last_match(0)
    end
  end

  def href_to_anchor(href, base_url, anchor_map)
    return nil if href.nil? || href.empty?
    return nil if href.start_with?("#")
    return nil if href =~ /\A[a-zA-Z][a-zA-Z0-9+.\-]*:/ # http:, mailto:, tel:, data:, ...

    path = href.split("#", 2).first
    return nil if path.nil? || path.empty?

    abs =
      if path.start_with?("/")
        path
      else
        begin
          URI.join("http://h" + (base_url || "/"), path).path
        rescue StandardError
          nil
        end
      end
    return nil unless abs

    anchor = anchor_map[norm(abs)]
    anchor ? "##{anchor}" : nil
  end

  # Embed referenced /images/* assets as base64 data URIs (best-effort).
  def embed_images(site, html)
    html.gsub(/(src=)("|')(\/images\/[^"'>\s]+)\2/) do
      prefix = Regexp.last_match(1)
      quote = Regexp.last_match(2)
      path = Regexp.last_match(3)
      data_uri = image_data_uri(site, path)
      data_uri ? %(#{prefix}#{quote}#{data_uri}#{quote}) : Regexp.last_match(0)
    end
  end

  def image_data_uri(site, path)
    file = site.in_source_dir(path.sub(%r{\A/}, ""))
    return nil unless File.file?(file)

    ext = File.extname(file).downcase
    mime = MIME_BY_EXT[ext] || "application/octet-stream"
    begin
      data = File.binread(file)
    rescue StandardError
      return nil
    end
    "data:#{mime};base64,#{Base64.strict_encode64(data)}"
  end

  def process_content(site, html, base_url, anchor_map)
    out = html.to_s.dup
    out = rewrite_links(out, base_url, anchor_map)
    out = embed_images(site, out)
    out
  end

  def build_html(site, lang)
    entries = site.data.dig("sidebars", "code", "entries") || []
    nodes = flatten_sidebar(entries, lang)
    index = build_index(site)[lang]

    anchor_map = {}
    nodes.each { |n| anchor_map[norm(n[:url])] = n[:anchor] if n[:url] }

    parts = []
    parts << %(<section class="pdf-cover"><div></div>) +
             %(<h1 class="pdf-cover__title">#{esc(DOC_TITLE[lang])}</h1>) +
             %(<div class="pdf-cover__date">#{esc(GEN_PREFIX[lang])} #{esc(generated_date(lang))}</div></section>)

    toc = [%(<nav class="pdf-toc"><h2 class="pdf-toc__title">#{esc(TOC_TITLE[lang])}</h2><ol class="pdf-toc__list">)]
    nodes.each do |n|
      margin = n[:level] * 16
      toc << %(<li class="pdf-toc__item pdf-toc__item--level-#{n[:level]}" style="margin-left:#{margin}px">) +
             %(<a href="##{n[:anchor]}">#{esc(n[:title])}</a></li>)
    end
    toc << "</ol></nav>"
    parts << toc.join

    nodes.each do |n|
      level = n[:level]
      parts << %(<article class="pdf-page pdf-page--level-#{level}" id="#{n[:anchor]}">)
      parts << %(<h1 class="pdf-page__title pdf-page__title--level-#{level}">#{esc(n[:title])}</h1>)
      if n[:url]
        page = index[norm(n[:url])]
        if page
          parts << process_content(site, page.content, page.url, anchor_map)
        else
          Jekyll.logger.warn "CodePrint:", "[#{lang}] no rendered page for #{n[:url]}"
        end
      end
      parts << "</article>"
    end

    body = parts.join("\n")

    <<~HTML
      <!DOCTYPE html>
      <html lang="#{lang}">
      <head>
      <meta charset="utf-8">
      <meta name="viewport" content="width=device-width, initial-scale=1.0">
      <title>#{esc(DOC_TITLE[lang])}</title>
      <style>
      #{PRINT_CSS}
      </style>
      </head>
      <body>
      #{body}
      </body>
      </html>
    HTML
  end

  class PrintPage < Jekyll::Page
    def initialize(site, lang)
      @site = site
      @base = site.source
      @dir = "#{lang}/print/code/documentation"
      @name = "index.html"

      self.process(@name)
      @path = site.in_source_dir(@dir, @name)

      self.data = {
        "layout" => "none",
        "title" => CodePrint::DOC_TITLE[lang],
        "lang" => lang,
        "product_code" => "code",
        "is_code_print" => true,
        "searchable" => false,
        "sitemap_include" => false,
        "feedback" => false,
        "output" => "web",
      }
      self.content = ""

      Jekyll::Hooks.trigger :pages, :post_init, self
    end
  end

  class Generator < Jekyll::Generator
    safe true
    priority :low

    def generate(site)
      CodePrint::LANGS.each do |lang|
        site.pages << PrintPage.new(site, lang)
      end
    end
  end
end

# After all pages are rendered, `page.content` holds the converted HTML. Stitch
# the code pages into each print page's final output (bypassing the layout).
Jekyll::Hooks.register :site, :post_render do |site|
  print_pages = site.pages.select { |p| p.data["is_code_print"] }
  next if print_pages.empty?

  print_pages.each do |print_page|
    lang = print_page.data["lang"]
    next unless CodePrint::LANGS.include?(lang)

    print_page.output = CodePrint.build_html(site, lang)
  end
end
