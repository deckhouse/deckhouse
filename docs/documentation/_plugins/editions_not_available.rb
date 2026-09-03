# Excludes pages and FAQ collection documents whose front matter
# `editionsNotAvailable` contains the current site.config['d8Revision']
# (case-insensitive). Also removes matching page URLs from static sidebar YAML
# so links to excluded pages are not shown.
#
# FAQ items (collections.faq, output: false) are listed via faq-list.liquid and
# search.liquid from site.faq; removing them from the collection excludes them
# from both.
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

def document_unavailable_in_edition?(document, current_edition)
  unavailable_editions =
    Array(document.data['editionsNotAvailable'])
      .map { |edition| edition.to_s.strip.downcase }

  unavailable_editions.include?(current_edition)
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
  excluded_pages = 0
  excluded_faqs = 0

  site.pages.delete_if do |page|
    next false unless document_unavailable_in_edition?(page, current_edition)

    url = normalize_editions_not_available_url(page.url)
    excluded_urls[url] = true if url
    excluded_pages += 1
    true
  end

  faq_collection = site.collections['faq']
  if faq_collection
    faq_collection.docs.delete_if do |doc|
      next false unless document_unavailable_in_edition?(doc, current_edition)

      excluded_faqs += 1
      true
    end
  end

  next if excluded_pages.zero? && excluded_faqs.zero?

  puts "Custom hook: excluding content unavailable in edition #{site.config['d8Revision']} " \
       "(#{excluded_pages} page(s), #{excluded_faqs} FAQ item(s))"

  filter_sidebars_editions_not_available!(site.data['sidebars'], excluded_urls) unless excluded_urls.empty?
end
