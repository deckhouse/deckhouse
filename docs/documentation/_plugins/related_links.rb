require 'nokogiri'
require 'uri'
require 'liquid'

module Jekyll
  class LinksExtractor
    # List of domains to skip when extracting links
    SKIP_DOMAINS = %w[
      example.com
      example.org
      example.net
      test.com
      localhost
      127.0.0.1
    ].freeze
    def self.extract_links_from_content(content, base_url = '', site_data = nil, page_lang = 'en', jekyll_context = nil)
      return [] unless content

      links = []

      # Extract markdown links [text](url)
      content.scan(/\[([^\]]*)\]\(([^)]+)\)/) do |text, url|
        next if skip_link?(url)

        # Render Jekyll expressions in URL if present
        final_url = has_jekyll_expressions?(url) ? render_jekyll_url(url, jekyll_context) : url

        link_type = determine_link_type(final_url, final_url)

        # Determine title based on link type
        title = text.strip
        if link_type == 'module_doc' || link_type == 'module_conf' || link_type == 'module_crds' || link_type == 'module_cluster_conf'
          module_name = extract_module_name(final_url)
          if module_name && site_data && site_data['i18n'] && site_data['i18n']['common']
            case link_type
            when 'module_conf'
              template = site_data['i18n']['common']['module_x_parameters'][page_lang]
              title = template&.gsub('XXXX', module_name) || "Module #{module_name} configuration"
            when 'module_cluster_conf'
              template = site_data['i18n']['common']['module_x_cluster_configuration'][page_lang]
              title = template&.gsub('XXXX', module_name) || "Module #{module_name} provider configuration"
            when 'module_crds'
              template = site_data['i18n']['common']['module_x_crds'][page_lang]
              title = template&.gsub('XXXX', module_name) || "Module #{module_name} custom resources"
            when 'module_doc'
              template = site_data['i18n']['common']['module_x_documentation'][page_lang]
              title = template&.gsub('XXXX', module_name) || "Module #{module_name} documentation"
            end
          end
        elsif link_type == 'global_crds' || link_type == 'global_conf'
          # Extract resource name from global reference URL
          resource_name = extract_global_resource_name(url)
          if resource_name
            title = "Global #{resource_name} custom resource"
          else
            # Use translation based on link type
            if link_type == 'global_crds'
              title = site_data['i18n']['common']['global_crds'][page_lang]
            elsif link_type == 'global_conf'
              title = site_data['i18n']['common']['global_parameters'][page_lang]
            end
          end
        else
          # Skip links which is neither module nor global references
          # Maybe in future we will use such links too
          next
        end

        # For module_crds, module_conf, module_cluster_conf, global_crds, and global_conf links, remove anchors from URL
        if link_type == 'module_crds' || link_type == 'module_conf' || link_type == 'module_cluster_conf' || link_type == 'global_crds' || link_type == 'global_conf'
          final_url = final_url.split('#')[0]
        end

        # For module_docs, use only base module URL (e.g., /modules/cloud-provider-aws/faq.html -> /modules/cloud-provider-aws/)
        if link_type == 'module_doc'
          # Extract module name and construct base module URL
          module_name = extract_module_name(final_url)
          if module_name
            # Remove language prefix and construct base module URL
            base_url = final_url.sub(/^(\/?(en\/|ru\/))?/, '')
            final_url = "/modules/#{module_name}/"
          end
        end

        link_data = {
          'url' => final_url,
          'title' => title,
          'type' => link_type
        }

        # Add module name for module links
        if link_type == 'module_doc' || link_type == 'module_conf' || link_type == 'module_crds' || link_type == 'module_cluster_conf'
          module_name = extract_module_name(final_url)
          link_data['module'] = module_name if module_name
        end

        links << link_data
      end

      # Extract HTML links <a href="url">text</a>
      begin
        doc = Nokogiri::HTML::DocumentFragment.parse(content)
        doc.css('a[href]').each do |link|
          url = link['href']
          next if skip_link?(url)

          # Render Jekyll expressions in URL if present
          final_url = has_jekyll_expressions?(url) ? render_jekyll_url(url, jekyll_context) : url

          title = link.text.strip
          title = link['title'] if title.empty? && link['title']
          title = final_url if title.empty?

        link_type = determine_link_type(final_url, final_url)

        # Determine title based on link type
        if link_type == 'module_doc' || link_type == 'module_conf' || link_type == 'module_crds' || link_type == 'module_cluster_conf'
          module_name = extract_module_name(final_url)
          if module_name && site_data && site_data['i18n'] && site_data['i18n']['common']
            case link_type
            when 'module_conf'
              template = site_data['i18n']['common']['module_x_parameters'][page_lang]
              title = template&.gsub('XXXX', module_name) || "Module #{module_name} configuration"
            when 'module_cluster_conf'
              template = site_data['i18n']['common']['module_x_cluster_configuration'][page_lang]
              title = template&.gsub('XXXX', module_name) || "Module #{module_name} provider configuration"
            when 'module_crds'
              template = site_data['i18n']['common']['module_x_crds'][page_lang]
              title = template&.gsub('XXXX', module_name) || "Module #{module_name} custom resources"
            when 'module_doc'
              template = site_data['i18n']['common']['module_x_documentation'][page_lang]
              title = template&.gsub('XXXX', module_name) || "Module #{module_name} documentation"
            end
          end
        elsif link_type == 'global_crds' || link_type == 'global_conf'
          # Extract resource name from global reference URL
          resource_name = extract_global_resource_name(url)
          if resource_name
            title = "Global #{resource_name} custom resource"
          else
            # Use translation based on link type
            if site_data && site_data['i18n'] && site_data['i18n']['common']
              if link_type == 'global_crds'
                title = site_data['i18n']['common']['global_crds'][page_lang] || "Global custom resources"
              elsif link_type == 'global_conf'
                title = site_data['i18n']['common']['global_parameters'][page_lang] || "Global parameters"
              end
            end
          end
        end

        # For module_crds, module_conf, module_cluster_conf, global_crds, and global_conf links, remove anchors from URL
        if link_type == 'module_crds' || link_type == 'module_conf' || link_type == 'module_cluster_conf' || link_type == 'global_crds' || link_type == 'global_conf'
          final_url = final_url.split('#')[0]
        end

        # For module_docs, use only base module URL (e.g., /modules/cloud-provider-aws/faq.html -> /modules/cloud-provider-aws/)
        if link_type == 'module_doc'
          # Extract module name and construct base module URL
          module_name = extract_module_name(final_url)
          if module_name
            # Remove language prefix and construct base module URL
            base_url = final_url.sub(/^(\/?(en\/|ru\/))?/, '')
            final_url = "/modules/#{module_name}/"
          end
        end

        link_data = {
          'url' => final_url,
          'title' => title,
          'type' => link_type
        }

        # Add module property for module links
        if link_type == 'module_doc' || link_type == 'module_conf' || link_type == 'module_crds' || link_type == 'module_cluster_conf'
          module_name = extract_module_name(final_url)
          link_data['module'] = module_name if module_name
        end

        links << link_data
        end
      rescue => e
        puts "Warning: Error parsing HTML in content: #{e.message}"
      end

      # Remove duplicates and return
      links.uniq { |link| link['url'] }
    end

    private

    def self.has_jekyll_expressions?(url)
      # Check if URL contains Jekyll/Liquid expressions
      url.include?('{{') || url.include?('{%')
    end

    def self.render_jekyll_url(url, jekyll_context)
      return url unless jekyll_context

      begin
        # Parse and render the Liquid template using the global Jekyll context
        template = Liquid::Template.parse(url)
        rendered_url = template.render(jekyll_context)

        # Return the rendered URL, or original if rendering failed
        rendered_url.empty? ? url : rendered_url
      rescue => e
        puts "Warning: Failed to render Jekyll expression in URL '#{url}': #{e.message}"
        url
      end
    end

    def self.skip_link?(url)
      return true if url.nil? || url.empty?

      # Skip mailto links
      return true if url.start_with?('mailto:')

      # Skip anchor links (fragments only)
      return true if url.start_with?('#')

      # Skip asset files
      asset_extensions = %w[.jpg .jpeg .png .gif .svg .ico .webp .bmp .tiff .css .js .json .xml .pdf .zip .tar .gz .mp4 .mp3 .wav .avi .mov .wmv .flv .webm .ogg .woff .woff2 .ttf .eot .otf]
      return true if asset_extensions.any? { |ext| url.downcase.end_with?(ext) }

      # Skip data URLs
      return true if url.start_with?('data:')

      # Skip javascript: and other non-document protocols
      return true if url.match?(/^javascript:/)

      # Skip external links to domains in the skip list
      if url.match?(/^https?:\/\//)
        begin
          uri = URI.parse(url)
          domain = uri.host&.downcase
          return true if domain && SKIP_DOMAINS.any? { |skip_domain| domain == skip_domain || domain.end_with?(".#{skip_domain}") }
        rescue URI::InvalidURIError
          # If URL parsing fails, continue with normal processing
        end
      end

      # Skip domain-only links (without protocol) that match skip list
      if url.match?(/^[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/) && !url.include?('/')
        domain = url.downcase
        return true if SKIP_DOMAINS.any? { |skip_domain| domain == skip_domain || domain.end_with?(".#{skip_domain}") }
      end

      false
    end

    def self.determine_link_type(original_url, normalized_url)
      # Check for external links (begins with a protocol)
      if original_url.match?(/^[a-zA-Z][a-zA-Z0-9+.-]*:/)
        return 'external_doc'
      end

      # Ensure we have a leading slash for pattern matching
      url_for_matching = normalized_url.start_with?('/') ? normalized_url : "/#{normalized_url}"

      # For module URLs, remove anchors to treat them as the same URL
      if url_for_matching.match?(%r{/modules/[^/]+/})
        url_for_matching = url_for_matching.split('#')[0]
      end

      # Check for module configuration links
      if url_for_matching.match?(%r{/modules/[^/]+/configuration.*\.html.*$})
        return 'module_conf'
      end

      # Check for module cluster configuration links
      if url_for_matching.match?(%r{/modules/[^/]+/cluster_configuration\.html.*$})
        return 'module_cluster_conf'
      end

      # Check for module CR links
      if url_for_matching.match?(%r{/modules/[^/]+/cr\.html.*$})
        return 'module_crds'
      end

      # Check for module documentation links
      if url_for_matching.match?(%r{/modules/[^/]+/})
        return 'module_doc'
      end

      # Check for global CR links
      if url_for_matching.match?(%r{/reference/api/cr\.html.*})
        return 'global_crds'
      end

      # Check for global configuration links
      if url_for_matching.match?(%r{/reference/api/global\.html.*})
        return 'global_conf'
      end

      # Default to internal document
      'internal_doc'
    end

    def self.extract_module_name(url)
      # Extract module name from module URLs
      # Remove anchors for consistent matching
      clean_url = url.split('#')[0]

      # Handle different URL formats: /modules/name/, modules/name/, /en/modules/name/, en/modules/name/
      match = clean_url.match(%r{^(\/?(en|ru)\/)?\/?modules\/([^/]+)\/})
      match ? match[3] : nil
    end

    def self.extract_global_resource_name(url)
      # Extract resource name from global reference URLs
      return nil
      # TODO: refactor this to get the CamelCase name.
      match = clean_url.match(%r{/reference/api/cr\.html\#([a-z]+).*\.html$})
      match ? match[1] : nil
    end

  end
end

Jekyll::Hooks.register :site, :pre_render do |site|
  puts "Extracting related links..."

  site.pages.each do |page|
    next unless page.content

    # Skip pages with embedded_modules sidebar
    next if page.data['sidebar'] == 'embedded-modules'

    # Get the base URL for this page (without the filename)
    base_url = page.url.sub(/\/[^\/]*$/, '')
    base_url = base_url[1..-1] if base_url.start_with?('/')

    # Remove language prefix from base URL (en/ or ru/)
    base_url = base_url.sub(/^(en\/|ru\/)/, '')

    # Extract links from the page content
    page_lang = page['lang'] || 'en'

    # Create Jekyll context for rendering
    jekyll_context = {
      'site' => {
        'mode' => site.config['mode'],
        'd8Revision' => site.config['d8Revision'],
        'urls' => site.config['urls']
      },
      'page' => {
        'lang' => page_lang
      }
    }

    extracted_links = Jekyll::LinksExtractor.extract_links_from_content(page.content, base_url, site.data, page_lang, jekyll_context)

    # Get existing related_links from page metadata
    existing_links = page.data['related_links'] || []

    # Validate existing_links structure and add type if missing
    valid_existing_links = []
    if existing_links.any?
      begin
        existing_links.each do |link|
          if link.is_a?(Hash) && link.key?('url') && !link['url'].to_s.strip.empty?
            # Create a copy of the link to avoid modifying the original
            processed_link = link.dup

            # Add type if missing
            unless processed_link.key?('type')
              link_type = Jekyll::LinksExtractor.determine_link_type(processed_link['url'], processed_link['url'])
              processed_link['type'] = link_type
            end

            # Add module property and standardized title for module links if missing
            if processed_link['type'] == 'module_doc' || processed_link['type'] == 'module_conf' || processed_link['type'] == 'module_crds' || processed_link['type'] == 'module_cluster_conf'
              module_name = Jekyll::LinksExtractor.extract_module_name(processed_link['url'])
              if module_name
                # Add module property if missing
                processed_link['module'] = module_name unless processed_link.key?('module')

                # Update title to standardized format using translations
                if site.data && site.data['i18n'] && site.data['i18n']['common']
                  case processed_link['type']
                  when 'module_conf'
                    template = site.data['i18n']['common']['module_x_parameters'][page_lang]
                    processed_link['title'] = template&.gsub('XXXX', module_name) || "Module #{module_name} configuration"
                  when 'module_cluster_conf'
                    template = site.data['i18n']['common']['module_x_cluster_configuration'][page_lang]
                    processed_link['title'] = template&.gsub('XXXX', module_name) || "Module #{module_name} provider configuration"
                  when 'module_crds'
                    template = site.data['i18n']['common']['module_x_crds'][page_lang]
                    processed_link['title'] = template&.gsub('XXXX', module_name) || "Module #{module_name} custom resources"
                  when 'module_doc'
                    template = site.data['i18n']['common']['module_x_documentation'][page_lang]
                    processed_link['title'] = template&.gsub('XXXX', module_name) || "Module #{module_name} documentation"
                  end
                end
              end
            elsif processed_link['type'] == 'global_crds' || processed_link['type'] == 'global_conf'
              # Update title for global reference links
              resource_name = Jekyll::LinksExtractor.extract_global_resource_name(processed_link['url'])
              if resource_name
                processed_link['title'] = "Global #{resource_name} custom resource"
              else
                # Use translation based on link type
                if processed_link['type'] == 'global_crds'
                  processed_link['title'] = site.data['i18n']['common']['global_crds'][page_lang] || "Global custom resources"
                elsif processed_link['type'] == 'global_conf'
                  processed_link['title'] = site.data['i18n']['common']['global_parameters'][page_lang] || "Global parameters"
                end
              end
            end

            # For module_crds, module_conf, module_cluster_conf, global_crds, and global_conf links, remove anchors from URL
            if processed_link['type'] == 'module_crds' || processed_link['type'] == 'module_conf' || processed_link['type'] == 'module_cluster_conf' || processed_link['type'] == 'global_crds' || processed_link['type'] == 'global_conf'
              processed_link['url'] = processed_link['url'].split('#')[0]
            end

            valid_existing_links << processed_link
          else
            puts "Warning: Skip link with invalid structure in related_links for #{page.url}: #{link.inspect}"
          end
        end

      rescue => e
        puts "Warning: Error processing related_links for #{page.url}: #{e.message}. Using only extracted_links."
        valid_existing_links = []
      end
    end

    page.data['related_links'] = valid_existing_links

    # Remove items from extracted_links if there is an item with the same url in related_links
    valid_existing_links_urls = valid_existing_links.map { |link| link['url'] }
    extracted_links = extracted_links.reject { |link| valid_existing_links_urls.include?(link['url']) }

    # If there are items with the same module and one has type 'module_doc', keep only the 'module_doc' item
    extracted_links = extracted_links.group_by { |link| link['module'] }.flat_map do |module_name, links|
      if module_name && links.any? { |link| link['type'] == 'module_doc' }
        # Keep only the module_doc item for this module
        links.select { |link| link['type'] == 'module_doc' }
      else
        # Keep all items if no module_doc exists or no module name
        links
      end
    end

    # Limit extracted_links to the first extracted_links_max items if specified
    if valid_existing_links.size > 0
      if page.data['extracted_links_max'] && page.data['extracted_links_max'].is_a?(Integer) && page.data['extracted_links_max'] >= 0
        max_links = page.data['extracted_links_max']
      else
        max_links = 2
      end
    else
      if page.data['extracted_links_only_max'] && page.data['extracted_links_only_max'].is_a?(Integer) && page.data['extracted_links_only_max'] >= 0
        max_links = page.data['extracted_links_only_max']
      else
        max_links = 6
      end
    end
    extracted_links = extracted_links.first(max_links)

    # Sort extracted_links: first global_conf/global_crds, then others sorted by module
    extracted_links = extracted_links.sort_by do |link|
      if link['type'] == 'global_conf' || link['type'] == 'global_crds'
        [0, '']  # Global links come first
      else
        [1, link['module'] || '']  # Other links sorted by module
      end
    end

    page.data['extracted_links'] = extracted_links

  end

  puts "Finished extracting related links..."
end
