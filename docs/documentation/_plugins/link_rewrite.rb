# Rewrites <a href> links in the final HTML.
#
# Controlled by site.config:
#   linkRewriteEnable: true|false  # global on/off switch
#   linkRewrite:
#     whitelistHosts: [host1, host2]  # links NOT on these hosts (or their subdomains) are stripped
#     replace:                         # regex URL substitutions, applied to ANY link
#       - { from: 'regex', to: 'replacement' }
#     remove:                          # regex patterns; matching links are unwrapped
#       - 'regex'                      # (applied AFTER replace, BEFORE whitelist)
#
# Per-link opt-out: <a data-keep-link="true"> is never touched.
# In Markdown: [text](https://example.com){:data-keep-link="true"}
#
# Flow: for every <a href>:
#   1. If data-keep-link is set — skip.
#   2. Apply replace rules iteratively (any link — relative or absolute): the first
#      matching rule is applied and the scan restarts, until nothing changes.
#      A hard pass cap and cycle detection prevent infinite loops.
#   3. If the (possibly rewritten) href matches any 'remove' pattern — unwrap the <a>.
#   4. Otherwise, if the href is absolute http(s), run the whitelist check —
#      unwrap the <a> when the host is not in whitelistHosts (or a subdomain of one).
#   5. Non-http(s) URLs (relative, mailto:, tel:, #anchor) that survived step 3
#      are kept as is (with the replace result if any rule matched).
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
    site.config['linkRewriteEnable'] == true
  end

  def html_output?(doc)
    doc.output_ext == '.html'
  end

  def compile_rules(site)
    cfg = site.config['linkRewrite'] || {}
    whitelist = Array(cfg['whitelistHosts']).map { |h| h.to_s.downcase }
    replaces = Array(cfg['replace']).map do |r|
      [Regexp.new(r['from'].to_s), r['to'].to_s]
    end
    removes = Array(cfg['remove']).map { |p| Regexp.new(p.to_s) }
    [whitelist, replaces, removes]
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

  MAX_REPLACE_PASSES = 5

  # Applies the first matching replace rule once. Returns href unchanged if none match.
  def apply_first_matching_replace(href, replaces)
    replaces.each do |regex, replacement|
      new_href = href.sub(regex, replacement)
      return new_href if new_href != href
    end
    href
  end

  # Applies replace rules iteratively: the first matching rule is applied and the
  # scan restarts from the top, until a full pass changes nothing (fixpoint).
  # Guarded against infinite loops by a hard pass cap and by cycle detection
  # (a repeated href value means the rules ping-pong). If neither converges, warn
  # and keep the last value.
  def apply_replaces(href, replaces)
    result = href
    seen = { result => true }
    MAX_REPLACE_PASSES.times do
      new_result = apply_first_matching_replace(result, replaces)
      return result if new_result == result # converged: no rule matched

      result = new_result
      if seen.key?(result)
        Jekyll.logger.warn 'linkRewrite:', "replace rules cycle on #{href.inspect}; stopping"
        return result
      end
      seen[result] = true
    end
    Jekyll.logger.warn 'linkRewrite:', "replace rules did not converge for #{href.inspect} after #{MAX_REPLACE_PASSES} passes"
    result
  end

  def process(doc, whitelist, replaces, removes)
    return if doc.output.nil? || doc.output.empty?

    doc.output = doc.output.gsub(A_TAG_RE) do |match|
      attrs = Regexp.last_match(1)
      inner = Regexp.last_match(2)

      next match if attrs =~ KEEP_RE

      href_match = attrs.match(HREF_RE)
      next match unless href_match
      href = href_match[1] || href_match[2]

      # Apply replace rules to ANY link (relative or absolute).
      new_href = apply_replaces(href, replaces)
      if new_href != href
        attrs = attrs.sub(href_match[0], %(href="#{new_href}"))
      end

      # Explicit removal by URL pattern — applied to ANY link (relative or absolute).
      next inner if removes.any? { |re| new_href.match?(re) }

      # Whitelist check applies only to absolute http(s) URLs. Anything else
      # (relative, mailto:, tel:, #anchor) is kept as is.
      new_host = host_of(new_href)
      if new_host.nil? || whitelisted?(new_host, whitelist)
        next "<a#{attrs}>#{inner}</a>"
      end

      inner
    end
  end
end

Jekyll::Hooks.register [:documents, :pages], :post_render do |doc|
  site = doc.site
  next unless LinkRewrite.enabled?(site)
  next unless LinkRewrite.html_output?(doc)

  whitelist, replaces, removes = LinkRewrite.compile_rules(site)
  LinkRewrite.process(doc, whitelist, replaces, removes)
end
