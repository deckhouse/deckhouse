module Jekyll
  class NavigationHelper
    def self.normalize_sidebar_url(url)
      return nil if url.nil?
      normalized = url.to_s.strip
      return nil if normalized.empty?
      normalized = normalized.sub(%r{^/(en|ru)/}, '/')
      normalized = normalized.sub(%r{/index\.html?$}, '/')
      normalized
    end

    def self.find_breadcrumb_titles(entries, target_url, lang = 'en', parent_titles = [])
      return nil if entries.nil? || !entries.is_a?(Array)

      normalized_target = normalize_sidebar_url(target_url)
      return nil if normalized_target.nil?

      entries.each do |entry|
        next if entry['draft'] == true

        title = entry.dig('title', lang) || entry['title']
        next if title.nil? || title.to_s.strip.empty?

        entry_url = normalize_sidebar_url(entry['url'])
        return parent_titles if entry_url && entry_url == normalized_target

        if entry['folders'].is_a?(Array)
          found = find_breadcrumb_titles(entry['folders'], normalized_target, lang, parent_titles + [title])
          return found if found
        end
      end

      nil
    end

    def self.find_breadcrumb_titles_for_page(site, page, sidebar_name = 'main')
      return [] unless site.data['sidebars'][sidebar_name]

      lang = page['lang'] || 'en'
      entries = site.data['sidebars'][sidebar_name]['entries']
      target_url = page['url']

      breadcrumbs = find_breadcrumb_titles(entries, target_url, lang, []) || []

      # Remove adjacent duplicates.
      deduplicated = []
      breadcrumbs.each do |item|
        next if deduplicated.last == item
        deduplicated << item
      end

      # If last breadcrumb matches current page title, skip it.
      page_title = (page['title'] || '').to_s.strip.downcase
      if !deduplicated.empty? && deduplicated.last.to_s.strip.downcase == page_title
        deduplicated.pop
      end

      deduplicated
    end

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

    def self.embedded_module_entry_title(site, entry, lang = 'en')
      return nil unless entry && entry['url']

      if entry['url'].end_with?('/')
        site.data.dig('modules', 'sidebar', 'titles', 'overview', lang)
      else
        site.data.dig('modules', 'sidebar', 'titles', File.basename(entry['url']), lang) || entry.dig('title', lang)
      end
    end

    def self.flatten_embedded_module_entries(site, page, lang = 'en')
      module_name = page['module-kebab-name'] || page['moduleName']
      return [] if module_name.nil? || module_name.to_s.strip.empty?

      module_sidebar = site.data.dig('sidebars', 'embedded-modules', module_name)
      return [] unless module_sidebar && module_sidebar['folders'].is_a?(Array)

      module_title = module_name
      flattened = []

      module_sidebar['folders'].sort_by { |entry| entry['weight'] || 100 }.each do |entry|
        next if entry['draft'] == true

        title = embedded_module_entry_title(site, entry, lang)
        next if title.nil? || title.to_s.strip.empty?

        display_title = module_title.to_s.strip.empty? ? title : "#{module_title} / #{title}"

        flattened << {
          'title' => title,
          'display_title' => display_title,
          'url' => entry['url'],
          'external_url' => entry['external_url']
        }
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

    def self.get_embedded_module_url(path, current_page_url)
      return nil if path.nil?

      normalized_path = path.start_with?('/') ? path : "/#{path}"
      current_url = current_page_url.to_s.sub(%r{^/(en|ru)/}, '/')
      channel_pattern = '(?:v[0-9]+\.[0-9]+|alpha|beta|early-access|stable|rock-solid|latest)'
      current_match = current_url.match(%r{\A/modules/([^/]+)/(#{channel_pattern})(?:/.*)?\z})

      return normalized_path unless current_match

      module_name = current_match[1]
      channel = current_match[2]
      module_prefix = "/modules/#{module_name}/"

      return normalized_path unless normalized_path.start_with?(module_prefix)

      entry_suffix = normalized_path.sub(module_prefix, '')
      return normalized_path if entry_suffix.match?(%r{\A#{channel_pattern}(?:/|\z)})

      "/modules/#{module_name}/#{channel}/#{entry_suffix}"
    end

    def self.find_navigation_pages(site, page, sidebar_name = 'main')
      return { 'prev' => nil, 'next' => nil } unless site.data['sidebars'][sidebar_name]

      lang = page['lang'] || 'en'
      flattened = if sidebar_name == 'embedded-modules'
                    flatten_embedded_module_entries(site, page, lang)
                  else
                    entries = site.data['sidebars'][sidebar_name]['entries']
                    flatten_sidebar_entries(entries, lang)
                  end

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
        prev_page['full_url'] = if sidebar_name == 'embedded-modules'
                                  get_embedded_module_url(prev_page['url'], page['url'])
                                else
                                  get_relative_url(prev_page['url'], page['url'])
                                end
      end

      if next_page
        next_page['full_url'] = if sidebar_name == 'embedded-modules'
                                  get_embedded_module_url(next_page['url'], page['url'])
                                else
                                  get_relative_url(next_page['url'], page['url'])
                                end
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

    def get_breadcrumb_titles(page, sidebar_name = 'main')
      site = @context.registers[:site]
      Jekyll::NavigationHelper.find_breadcrumb_titles_for_page(site, page, sidebar_name)
    end
  end
end

Liquid::Template.register_filter(Jekyll::NavigationFilter)
