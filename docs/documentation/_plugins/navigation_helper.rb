module Jekyll
  class NavigationHelper
    def self.flatten_sidebar_entries(entries, lang = 'en', parent_titles = [])
      flattened = []

      return if entries.nil? || !entries.is_a?(Array)

      entries.each do |entry|
        next if entry['draft'] == true
        next unless entry.dig('title', lang)

        # Add current entry if it has a URL
        if entry['url']
          # Create display title with nearest parent context
          display_title = entry.dig('title', lang)
          if parent_titles.any?
            nearest_parent = parent_titles.last
            display_title = "#{nearest_parent} / #{display_title}"
          end

          flattened << {
            'title' => entry.dig('title', lang),
            'display_title' => display_title,
            'url' => entry['url'],
            'external_url' => entry['external_url']
          }
        end

        # Recursively process folders
        if entry['folders']
          new_parent_titles = parent_titles + [entry.dig('title', lang)]
          flattened.concat(flatten_sidebar_entries(entry['folders'], lang, new_parent_titles))
        end
      end

      flattened
    end

    def self.get_relative_url(path, current_page_url)
      # Remove first slash if exists
      page_path_relative = current_page_url.gsub(%r!^/!, "")
      page_depth = page_path_relative.scan(%r!/!).count - 1
      prefix = ""
      page_depth.times{ prefix = prefix + "../" }
      prefix + path.sub(%r!^/!, "./")
    end

    def self.find_navigation_pages(site, page, sidebar_name = 'main')
      return { 'prev' => nil, 'next' => nil } unless site.data['sidebars'][sidebar_name]

      lang = page['lang'] || 'en'
      entries = site.data['sidebars'][sidebar_name]['entries']
      flattened = flatten_sidebar_entries(entries, lang)

      return { 'prev' => nil, 'next' => nil } if flattened.nil? || flattened.empty?

      current_url = page['url'].sub(/\/index\.html?$/, '/')
      current_index = nil

      # Find current page index
      flattened.each_with_index do |entry, index|
        entry_url = entry['url']
        entry_with_lang = "/#{lang}#{entry_url}"

        if current_url == entry_url || current_url == entry_with_lang
          current_index = index
          break
        end
      end

      return { 'prev' => nil, 'next' => nil } if current_index.nil?

      # Get previous and next pages
      prev_page = current_index > 0 ? flattened[current_index - 1] : nil
      next_page = current_index < flattened.length - 1 ? flattened[current_index + 1] : nil

      # Convert to relative URLs
      if prev_page
        prev_page['full_url'] = get_relative_url(prev_page['url'], page['url'])
      end

      if next_page
        next_page['full_url'] = get_relative_url(next_page['url'], page['url'])
      end

      { 'prev' => prev_page, 'next' => next_page }
    end
  end

  # Liquid filter to get navigation pages
  module NavigationFilter
    def get_navigation_pages(page, sidebar_name = 'main')
      site = @context.registers[:site]
      Jekyll::NavigationHelper.find_navigation_pages(site, page, sidebar_name)
    end
  end
end

Liquid::Template.register_filter(Jekyll::NavigationFilter)
