# Rewrites <a href> links in the final HTML.
#
# Controlled by site.config:
#   link_rewrite_enable: true|false  # global on/off switch
#   link_rewrite:
#     whitelist_hosts: [host1, host2]  # links NOT on these hosts (or their subdomains) are stripped
#     replace:                         # regex URL substitutions, applied before whitelist check
#       - { from: 'regex', to: 'replacement' }
#
# Per-link opt-out: <a data-keep-link="true"> is never touched.
# In Markdown: [text](https://example.com){:data-keep-link="true"}
#
# Relative links and links without a host (mailto:, tel:, /path, #anchor) are always left alone.
#
# The plugin only rewrites <a> tags in-place via regex — the rest of the HTML is preserved
# byte-for-byte (no DOM re-serialization).

require 'uri'

module LinkRewrite
  # Non-greedy match of <a ...>...</a>. HTML5 forbids nested <a>, so this is safe.
  A_TAG_RE = /<a(\s[^>]*)>([\s\S]*?)<\/a>/i.freeze
  HREF_RE  = /\bhref\s*=\s*(?:"([^"]*)"|'([^']*)')/i.freeze
  KEEP_RE  = /\bdata-keep-link\s*=/i.freeze

  module_function

  def enabled?(site)
    site.config['link_rewrite_enable'] == true
  end

  def html_output?(doc)
    doc.output_ext == '.html'
  end

  def compile_rules(site)
    cfg = site.config['link_rewrite'] || {}
    whitelist = Array(cfg['whitelist_hosts']).map { |h| h.to_s.downcase }
    replaces = Array(cfg['replace']).map do |r|
      [Regexp.new(r['from'].to_s), r['to'].to_s]
    end
    [whitelist, replaces]
  end

  def host_of(href)
    URI.parse(href).host&.downcase
  rescue URI::InvalidURIError
    nil
  end

  def whitelisted?(host, whitelist)
    return false if host.nil?
    whitelist.any? { |w| host == w || host.end_with?(".#{w}") }
  end

  def apply_replaces(href, replaces)
    replaces.each do |regex, replacement|
      new_href = href.sub(regex, replacement)
      return new_href if new_href != href
    end
    href
  end

  def process(doc, whitelist, replaces)
    return if doc.output.nil? || doc.output.empty?

    doc.output = doc.output.gsub(A_TAG_RE) do |match|
      attrs = Regexp.last_match(1)
      inner = Regexp.last_match(2)

      next match if attrs =~ KEEP_RE

      href_match = attrs.match(HREF_RE)
      next match unless href_match
      href = href_match[1] || href_match[2]

      host = host_of(href)
      next match if host.nil?

      new_href = apply_replaces(href, replaces)
      if new_href != href
        new_host = host_of(new_href)
        if whitelisted?(new_host, whitelist)
          new_attrs = attrs.sub(href_match[0], %(href="#{new_href}"))
          next "<a#{new_attrs}>#{inner}</a>"
        end
      else
        next match if whitelisted?(host, whitelist)
      end

      inner
    end
  end
end

Jekyll::Hooks.register [:documents, :pages], :post_render do |doc|
  site = doc.site
  next unless LinkRewrite.enabled?(site)
  next unless LinkRewrite.html_output?(doc)

  whitelist, replaces = LinkRewrite.compile_rules(site)
  LinkRewrite.process(doc, whitelist, replaces)
end
