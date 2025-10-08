require 'nokogiri'
require 'uri'

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
    def self.extract_links_from_content(content, base_url = '', site_data = nil, page_lang = 'en')
      return [] unless content

      links = []

      # Extract markdown links [text](url)
      content.scan(/\[([^\]]*)\]\(([^)]+)\)/) do |text, url|
        next if skip_link?(url)

        link_type = determine_link_type(url, url)

        # Determine title based on link type
        title = text.strip
        if link_type == 'module_doc' || link_type == 'module_conf' || link_type == 'module_crds' || link_type == 'module_cluster_conf'
          module_name = extract_module_name(url)
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
        end

        # For module_crds, module_conf, module_cluster_conf, global_crds, and global_conf links, remove anchors from URL
        final_url = url
        if link_type == 'module_crds' || link_type == 'module_conf' || link_type == 'module_cluster_conf' || link_type == 'global_crds' || link_type == 'global_conf' || link_type == 'module_doc'
          final_url = url.split('#')[0]
        end

        link_data = {
          'url' => final_url,
          'title' => title,
          'type' => link_type
        }

        # Add module name for module links
        if link_type == 'module_doc' || link_type == 'module_conf' || link_type == 'module_crds' || link_type == 'module_cluster_conf'
          module_name = extract_module_name(url)
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

          title = link.text.strip
          title = link['title'] if title.empty? && link['title']
          title = url if title.empty?

        link_type = determine_link_type(url, url)

        # Determine title based on link type
        if link_type == 'module_doc' || link_type == 'module_conf' || link_type == 'module_crds' || link_type == 'module_cluster_conf'
          module_name = extract_module_name(url)
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
        final_url = url
        if link_type == 'module_crds' || link_type == 'module_conf' || link_type == 'module_cluster_conf' || link_type == 'global_crds' || link_type == 'global_conf'
          final_url = url.split('#')[0]
        end

        link_data = {
          'url' => final_url,
          'title' => title,
          'type' => link_type
        }

        # Add module property for module links
        if link_type == 'module_doc' || link_type == 'module_conf' || link_type == 'module_crds' || link_type == 'module_cluster_conf'
          module_name = extract_module_name(url)
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
    extracted_links = Jekyll::LinksExtractor.extract_links_from_content(page.content, base_url, site.data, page_lang)

    # Get existing related_links from page metadata
    existing_links = page.data['related_links'] || []

    # Validate existing_links structure and add type if missing
    valid_existing_links = []
    if existing_links.any?
      begin
        existing_links.each do |link|
          if link.is_a?(Hash) && link.key?('url') && link.key?('title')
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
            puts "Warning: Invalid link structure in related_links for #{page.url}: #{link.inspect}"
          end
        end

        if valid_existing_links.length != existing_links.length
          puts "Warning: Skipping malformed related_links for #{page.url}, using only extracted_links"
          valid_existing_links = []
        end
      rescue => e
        puts "Warning: Error processing related_links for #{page.url}: #{e.message}. Using only extracted_links."
        valid_existing_links = []
      end
    end

    # Merge extracted_links with valid existing_links and remove duplicates
    all_links = valid_existing_links + extracted_links
    merged_links = all_links.uniq { |link| link['url'] }

    # Store both extracted_links and merged related_links
    page.data['extracted_links'] = extracted_links
    page.data['related_links'] = merged_links

    # Debug output for pages with links
    # if extracted_links.any? || valid_existing_links.any?
    #   puts "  #{page.url}: Found #{extracted_links.length} extracted links, #{valid_existing_links.length} valid existing links, #{merged_links.length} total merged links"
    #   if extracted_links.any?
    #     puts "    Extracted links:"
    #     extracted_links.each do |link|
    #       puts "      - #{link['title']} -> #{link['url']} (#{link['type']})"
    #     end
    #   end
    #   if valid_existing_links.any?
    #     puts "    Existing links (cleaned URLs):"
    #     valid_existing_links.each do |link|
    #       puts "      - #{link['title']} -> #{link['url']} (#{link['type']})"
    #     end
    #   end
    # end
  end

  puts "Finished extracting related links..."
end
