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
    def self.extract_links_from_content(content, baseUrl = '', siteData = nil, pageLang = 'en', jekyllContext = nil)
      return [] unless content

      links = []

      # Extract markdown links [text](url)
      content.scan(/\[([^\]]*)\]\(([^)]+)\)/) do |text, url|
        next if skip_link?(url)

        # Render Jekyll expressions in URL if present
        finalUrl = has_jekyll_expressions?(url) ? render_jekyll_url(url, jekyllContext) : url

        # Skip if finalUrl still has Jekyll expressions
        next if has_jekyll_expressions?(finalUrl)

        linkType = determine_link_type(finalUrl, finalUrl)

        # Determine title based on link type
        title = text.strip
        if linkType == 'module_doc' || linkType == 'module_conf' || linkType == 'module_crds' || linkType == 'module_cluster_conf'
          moduleName = extract_module_name(finalUrl)
          if moduleName && siteData && siteData['i18n'] && siteData['i18n']['common']
            case linkType
            when 'module_conf'
              template = siteData['i18n']['common']['module_x_parameters'][pageLang]
              title = template&.gsub('XXXX', moduleName) || "Module #{moduleName} configuration"
            when 'module_cluster_conf'
              template = siteData['i18n']['common']['module_x_cluster_configuration'][pageLang]
              title = template&.gsub('XXXX', moduleName) || "Module #{moduleName} provider configuration"
            when 'module_crds'
              template = siteData['i18n']['common']['module_x_crds'][pageLang]
              title = template&.gsub('XXXX', moduleName) || "Module #{moduleName} custom resources"
            when 'module_doc'
              template = siteData['i18n']['common']['module_x_documentation'][pageLang]
              title = template&.gsub('XXXX', moduleName) || "Module #{moduleName} documentation"
            end
          end
        elsif linkType == 'global_crds' || linkType == 'global_conf'
          # Extract resource name from global reference URL
          resourceName = extract_global_resource_name(url)
          if resourceName
            title = "Global #{resourceName} custom resource"
          else
            # Use translation based on link type
            if linkType == 'global_crds'
              title = siteData['i18n']['common']['global_crds'][pageLang]
            elsif linkType == 'global_conf'
              title = siteData['i18n']['common']['global_parameters'][pageLang]
            end
          end
        else
          # Skip links which is neither module nor global references
          # Maybe in future we will use such links too
          next
        end

        # For module_crds, module_conf, module_cluster_conf, global_crds, and global_conf links, remove anchors from URL
        if linkType == 'module_crds' || linkType == 'module_conf' || linkType == 'module_cluster_conf' || linkType == 'global_crds' || linkType == 'global_conf'
          finalUrl = finalUrl.split('#')[0]
        end

        # For module_docs, use only base module URL (e.g., /modules/cloud-provider-aws/faq.html -> /modules/cloud-provider-aws/)
        if linkType == 'module_doc'
          # Extract module name and construct base module URL
          moduleName = extract_module_name(finalUrl)
          if moduleName
            # Remove language prefix and construct base module URL
            baseUrl = finalUrl.sub(/^(\/?(en\/|ru\/))?/, '')
            finalUrl = "/modules/#{moduleName}/"
          end
        end

        linkData = {
          'url' => finalUrl,
          'title' => title,
          'type' => linkType
        }

        # Add module name for module links
        if linkType == 'module_doc' || linkType == 'module_conf' || linkType == 'module_crds' || linkType == 'module_cluster_conf'
          moduleName = extract_module_name(finalUrl)
          linkData['module'] = moduleName if moduleName
        end

        links << linkData
      end

      # Extract HTML links <a href="url">text</a>
      begin
        doc = Nokogiri::HTML::DocumentFragment.parse(content)
        doc.css('a[href]').each do |link|
          url = link['href']
          next if skip_link?(url)

          # Render Jekyll expressions in URL if present
          finalUrl = has_jekyll_expressions?(url) ? render_jekyll_url(url, jekyllContext) : url

          # Skip if finalUrl still has Jekyll expressions
          next if has_jekyll_expressions?(finalUrl)

          title = link.text.strip
          title = link['title'] if title.empty? && link['title']
          title = finalUrl if title.empty?

          linkType = determine_link_type(finalUrl, finalUrl)

          # Determine title based on link type
          if linkType == 'module_doc' || linkType == 'module_conf' || linkType == 'module_crds' || linkType == 'module_cluster_conf'
            moduleName = extract_module_name(finalUrl)
            if moduleName && siteData && siteData['i18n'] && siteData['i18n']['common']
              case linkType
              when 'module_conf'
                template = siteData['i18n']['common']['module_x_parameters'][pageLang]
                title = template&.gsub('XXXX', moduleName) || "Module #{moduleName} configuration"
              when 'module_cluster_conf'
                template = siteData['i18n']['common']['module_x_cluster_configuration'][pageLang]
                title = template&.gsub('XXXX', moduleName) || "Module #{moduleName} provider configuration"
              when 'module_crds'
                template = siteData['i18n']['common']['module_x_crds'][pageLang]
                title = template&.gsub('XXXX', moduleName) || "Module #{moduleName} custom resources"
              when 'module_doc'
                template = siteData['i18n']['common']['module_x_documentation'][pageLang]
                title = template&.gsub('XXXX', moduleName) || "Module #{moduleName} documentation"
              end
            end
          elsif linkType == 'global_crds' || linkType == 'global_conf'
            # Extract resource name from global reference URL
            resourceName = extract_global_resource_name(url)
            if resourceName
              title = "Global #{resourceName} custom resource"
            else
              # Use translation based on link type
              if siteData && siteData['i18n'] && siteData['i18n']['common']
                if linkType == 'global_crds'
                  title = siteData['i18n']['common']['global_crds'][pageLang] || "Global custom resources"
                elsif linkType == 'global_conf'
                  title = siteData['i18n']['common']['global_parameters'][pageLang] || "Global parameters"
                end
              end
            end
          end

          # For module_crds, module_conf, module_cluster_conf, global_crds, and global_conf links, remove anchors from URL
          if linkType == 'module_crds' || linkType == 'module_conf' || linkType == 'module_cluster_conf' || linkType == 'global_crds' || linkType == 'global_conf'
            finalUrl = finalUrl.split('#')[0]
          end

          # For module_docs, use only base module URL (e.g., /modules/cloud-provider-aws/faq.html -> /modules/cloud-provider-aws/)
          if linkType == 'module_doc'
            # Extract module name and construct base module URL
            moduleName = extract_module_name(finalUrl)
            if moduleName
              # Remove language prefix and construct base module URL
              baseUrl = finalUrl.sub(/^(\/?(en\/|ru\/))?/, '')
              finalUrl = "/modules/#{moduleName}/"
            end
          end

          linkData = {
            'url' => finalUrl,
            'title' => title,
            'type' => linkType
          }

          # Add module property for module links
          if linkType == 'module_doc' || linkType == 'module_conf' || linkType == 'module_crds' || linkType == 'module_cluster_conf'
            moduleName = extract_module_name(finalUrl)
            linkData['module'] = moduleName if moduleName
          end

          links << linkData
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

    def self.render_jekyll_url(url, jekyllContext)
      return url unless jekyllContext

      begin
        # Parse and render the Liquid template using the global Jekyll context
        template = Liquid::Template.parse(url)
        renderedUrl = template.render(jekyllContext)

        # Return the rendered URL, or original if rendering failed
        renderedUrl.empty? ? url : renderedUrl
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
      assetExtensions = %w[.jpg .jpeg .png .gif .svg .ico .webp .bmp .tiff .css .js .json .xml .pdf .zip .tar .gz .mp4 .mp3 .wav .avi .mov .wmv .flv .webm .ogg .woff .woff2 .ttf .eot .otf]
      return true if assetExtensions.any? { |ext| url.downcase.end_with?(ext) }

      # Skip data URLs
      return true if url.start_with?('data:')

      # Skip javascript: and other non-document protocols
      return true if url.match?(/^javascript:/)

      # Skip external links to domains in the skip list
      if url.match?(/^https?:\/\//)
        begin
          uri = URI.parse(url)
          domain = uri.host&.downcase
          return true if domain && SKIP_DOMAINS.any? { |skipDomain| domain == skipDomain || domain.end_with?(".#{skipDomain}") }
        rescue URI::InvalidURIError
          # If URL parsing fails, continue with normal processing
        end
      end

      # Skip domain-only links (without protocol) that match skip list
      if url.match?(/^[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/) && !url.include?('/')
        domain = url.downcase
        return true if SKIP_DOMAINS.any? { |skipDomain| domain == skipDomain || domain.end_with?(".#{skipDomain}") }
      end

      false
    end

    def self.determine_link_type(originalUrl, normalizedUrl)
      # Check for external links (begins with a protocol)
      if originalUrl.match?(/^[a-zA-Z][a-zA-Z0-9+.-]*:/)
        return 'external_doc'
      end

      # Ensure we have a leading slash for pattern matching
      urlForMatching = normalizedUrl.start_with?('/') ? normalizedUrl : "/#{normalizedUrl}"

      # For module URLs, remove anchors to treat them as the same URL
      if urlForMatching.match?(%r{/modules/[^/]+/})
        urlForMatching = urlForMatching.split('#')[0]
      end

      # Check for module configuration links
      if urlForMatching.match?(%r{/modules/[^/]+/configuration.*\.html.*$})
        return 'module_conf'
      end

      # Check for module cluster configuration links
      if urlForMatching.match?(%r{/modules/[^/]+/cluster_configuration\.html.*$})
        return 'module_cluster_conf'
      end

      # Check for module CR links
      if urlForMatching.match?(%r{/modules/[^/]+/cr\.html.*$})
        return 'module_crds'
      end

      # Check for module documentation links
      if urlForMatching.match?(%r{/modules/[^/]+/})
        return 'module_doc'
      end

      # Check for global CR links
      if urlForMatching.match?(%r{/reference/api/cr\.html.*})
        return 'global_crds'
      end

      # Check for global configuration links
      if urlForMatching.match?(%r{/reference/api/global\.html.*})
        return 'global_conf'
      end

      # Default to internal document
      'internal_doc'
    end

    def self.extract_module_name(url)
      # Extract module name from module URLs
      # Remove anchors for consistent matching
      cleanUrl = url.split('#')[0]

      # Handle different URL formats: /modules/name/, modules/name/, /en/modules/name/, en/modules/name/
      match = cleanUrl.match(%r{^(\/?(en|ru)\/)?\/?modules\/([^/]+)\/})
      match ? match[3] : nil
    end

    def self.extract_global_resource_name(url)
      # Extract resource name from global reference URLs
      return nil
      # TODO: refactor this to get the CamelCase name.
      cleanUrl = url.split('#')[0]
      match = cleanUrl.match(%r{/reference/api/cr\.html\#([a-z]+).*\.html$})
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
    baseUrl = page.url.sub(/\/[^\/]*$/, '')
    baseUrl = baseUrl[1..-1] if baseUrl.start_with?('/')

    # Remove language prefix from base URL (en/ or ru/)
    baseUrl = baseUrl.sub(/^(en\/|ru\/)/, '')

    # Extract links from the page content
    pageLang = page['lang'] || 'en'

    # Create Jekyll context for rendering
    jekyllContext = {
      'site' => {
        'mode' => site.config['mode'],
        'd8Revision' => site.config['d8Revision'],
        'urls' => site.config['urls']
      },
      'page' => {
        'lang' => pageLang
      }
    }

    extractedLinks = Jekyll::LinksExtractor.extract_links_from_content(page.content, baseUrl, site.data, pageLang, jekyllContext)

    # Get existing relatedLinks from page metadata
    existingLinks = page.data['relatedLinks'] || []

    # Validate existingLinks structure and add type if missing
    validExistingLinks = []
    if existingLinks.any?
      begin
        existingLinks.each do |link|
          if link.is_a?(Hash) && link.key?('url') && !link['url'].to_s.strip.empty?
            # Create a copy of the link to avoid modifying the original
            processedLink = link.dup

            # Add type if missing
            unless processedLink.key?('type')
              linkType = Jekyll::LinksExtractor.determine_link_type(processedLink['url'], processedLink['url'])
              processedLink['type'] = linkType
            end

            # Add module property and standardized title for module links if missing
            if processedLink['type'] == 'module_doc' || processedLink['type'] == 'module_conf' || processedLink['type'] == 'module_crds' || processedLink['type'] == 'module_cluster_conf'
              moduleName = Jekyll::LinksExtractor.extract_module_name(processedLink['url'])
              if moduleName
                # Add module property if missing
                processedLink['module'] = moduleName unless processedLink.key?('module')

                # Update title to standardized format using translations
                if site.data && site.data['i18n'] && site.data['i18n']['common']
                  case processedLink['type']
                  when 'module_conf'
                    template = site.data['i18n']['common']['module_x_parameters'][pageLang]
                    processedLink['title'] = template&.gsub('XXXX', moduleName) || "Module #{moduleName} configuration"
                  when 'module_cluster_conf'
                    template = site.data['i18n']['common']['module_x_cluster_configuration'][pageLang]
                    processedLink['title'] = template&.gsub('XXXX', moduleName) || "Module #{moduleName} provider configuration"
                  when 'module_crds'
                    template = site.data['i18n']['common']['module_x_crds'][pageLang]
                    processedLink['title'] = template&.gsub('XXXX', moduleName) || "Module #{moduleName} custom resources"
                  when 'module_doc'
                    template = site.data['i18n']['common']['module_x_documentation'][pageLang]
                    processedLink['title'] = template&.gsub('XXXX', moduleName) || "Module #{moduleName} documentation"
                  end
                end
              end
            elsif processedLink['type'] == 'global_crds' || processedLink['type'] == 'global_conf'
              # Update title for global reference links
              resourceName = Jekyll::LinksExtractor.extract_global_resource_name(processedLink['url'])
              if resourceName
                processedLink['title'] = "Global #{resourceName} custom resource"
              else
                # Use translation based on link type
                if processedLink['type'] == 'global_crds'
                  processedLink['title'] = site.data['i18n']['common']['global_crds'][pageLang] || "Global custom resources"
                elsif processedLink['type'] == 'global_conf'
                  processedLink['title'] = site.data['i18n']['common']['global_parameters'][pageLang] || "Global parameters"
                end
              end
            end

            # For module_crds, module_conf, module_cluster_conf, global_crds, and global_conf links, remove anchors from URL
            if processedLink['type'] == 'module_crds' || processedLink['type'] == 'module_conf' || processedLink['type'] == 'module_cluster_conf' || processedLink['type'] == 'global_crds' || processedLink['type'] == 'global_conf'
              processedLink['url'] = processedLink['url'].split('#')[0]
            end

            validExistingLinks << processedLink
          else
            puts "Warning: Skip link with invalid structure in relatedLinks for #{page.url}: #{link.inspect}"
          end
        end

      rescue => e
        puts "Warning: Error processing relatedLinks for #{page.url}: #{e.message}. Using only extractedLinks."
        validExistingLinks = []
      end
    end

    page.data['relatedLinks'] = validExistingLinks

    # Remove items from extractedLinks if there is an item with the same url in relatedLinks
    validExistingLinksUrls = validExistingLinks.map { |link| link['url'] }
    extractedLinks = extractedLinks.reject { |link| validExistingLinksUrls.include?(link['url']) }

    # If there are items with the same module and one has type 'module_doc', keep only the 'module_doc' item
    extractedLinks = extractedLinks.group_by { |link| link['module'] }.flat_map do |moduleName, links|
      if moduleName && links.any? { |link| link['type'] == 'module_doc' }
        # Keep only the module_doc item for this module
        links.select { |link| link['type'] == 'module_doc' }
      else
        # Keep all items if no module_doc exists or no module name
        links
      end
    end

    # Limit extractedLinks to the first extractedLinksMax items if specified
    if validExistingLinks.size > 0
      if page.data['extractedLinksMax'] && page.data['extractedLinksMax'].is_a?(Integer) && page.data['extractedLinksMax'] >= 0
        maxLinks = page.data['extractedLinksMax']
      else
        maxLinks = 2
      end
    else
      if page.data['extractedLinksOnlyMax'] && page.data['extractedLinksOnlyMax'].is_a?(Integer) && page.data['extractedLinksOnlyMax'] >= 0
        maxLinks = page.data['extractedLinksOnlyMax']
      else
        maxLinks = 6
      end
    end
    extractedLinks = extractedLinks.first(maxLinks)

    # Sort extractedLinks: first global_conf/global_crds, then others sorted by module
    extractedLinks = extractedLinks.sort_by do |link|
      if link['type'] == 'global_conf' || link['type'] == 'global_crds'
        [0, '']  # Global links come first
      else
        [1, link['module'] || '']  # Other links sorted by module
      end
    end

    page.data['extractedLinks'] = extractedLinks

  end

  puts "Finished extracting related links..."
end
