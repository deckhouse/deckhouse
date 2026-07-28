# Excludes pages whose front matter `editionsNotAvailable` contains the current
# site.config['d8Revision'] (case-insensitive). Also removes matching entries
# from static sidebar YAML so links to excluded pages are not shown.
#
# Module sidebars generated from site.pages (sidebar_module.rb) are excluded
# automatically because they are built after this hook runs.

def normalize_editions_not_available_url(url)
  return nil if url.nil?

  normalized = url.to_s.strip
  return nil if normalized.empty?

  normalized = "/#{normalized}" unless normalized.start_with?('/')
  normalized = normalized.sub(%r{^/(en|ru)/}, '/')
  normalized.sub(%r{/index\.html?$}, '/')
end

def filter_sidebar_entries_editions_not_available(entries, excluded_urls)
  return [] unless entries.is_a?(Array)

  entries.filter_map do |entry|
    next unless entry.is_a?(Hash)

    if entry['folders'].is_a?(Array)
      entry['folders'] = filter_sidebar_entries_editions_not_available(entry['folders'], excluded_urls)
      if entry['folders'].empty?
        entry_url = normalize_editions_not_available_url(entry['url'])
        next if entry_url.nil? || excluded_urls[entry_url]
      end
    else
      entry_url = normalize_editions_not_available_url(entry['url'])
      next if entry_url && excluded_urls[entry_url]
    end

    entry
  end
end

def filter_sidebars_editions_not_available!(sidebars, excluded_urls)
  return unless sidebars.is_a?(Hash)

  sidebars.each_value do |sidebar|
    next unless sidebar.is_a?(Hash)
    next unless sidebar['entries'].is_a?(Array)

    sidebar['entries'] = filter_sidebar_entries_editions_not_available(sidebar['entries'], excluded_urls)
  end
end

Jekyll::Hooks.register :site, :post_read do |site|
  current_edition = site.config['d8Revision'].to_s.strip.downcase
  next if current_edition.empty?

  excluded_urls = {}

  site.pages.delete_if do |page|
    unavailable_editions =
      Array(page.data['editionsNotAvailable'])
        .map { |edition| edition.to_s.strip.downcase }

    next false unless unavailable_editions.include?(current_edition)

    url = normalize_editions_not_available_url(page.url)
    excluded_urls[url] = true if url
    true
  end

  next if excluded_urls.empty?

  puts "Custom hook: excluding pages unavailable in edition #{site.config['d8Revision']} (#{excluded_urls.size} URL(s))"

  filter_sidebars_editions_not_available!(site.data['sidebars'], excluded_urls)
end
