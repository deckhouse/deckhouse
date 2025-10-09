require 'json'

module Jekyll
  class SearchIndexGenerator < Generator
    safe true
    priority :low

    def generate(site)
      puts "SearchIndexGenerator: Generating search index files..."

      # Check if embedded_modules mode is enabled
      embedded_modules_mode = site.config['embedded_modules'] == true
      puts "SearchIndexGenerator: Embedded modules mode enabled: #{embedded_modules_mode}"

      if embedded_modules_mode
        puts "Embedded modules mode enabled - generating only embedded modules search indices"
        # Generate only embedded modules search index data
        generate_embedded_modules_search_index_data(site)

        # Generate search-embedded-modules-index.json files for all languages
        embedded_modules_indices = site.data['search_embedded_modules_indices']
        if embedded_modules_indices
          embedded_modules_indices.each do |lang, embedded_data|
            filename = lang == 'en' ? 'search-embedded-modules-index.json' : "search-embedded-modules-index-#{lang}.json"
            embedded_page = PageWithoutAFile.new(site, site.source, '', filename)
            embedded_page.content = JSON.pretty_generate(embedded_data)
            embedded_page.data = {
              'layout' => 'none',
              'searchable' => false,
              'sitemap_include' => false,
              'permalink' => "#{lang}/search-embedded-modules-index.json"
            }
            embedded_page.data['lang'] = lang unless lang == 'en'
            site.pages << embedded_page
          end
        end
      else
        puts "Standard mode - generating standard search indices only"
        # Generate search index data first
        generate_search_index_data(site)

        # Generate search.json files for all languages
        search_indices = site.data['search_indices']
        if search_indices
          search_indices.each do |lang, search_data|
            filename = lang == 'en' ? 'search.json' : "search-#{lang}.json"
            search_page = PageWithoutAFile.new(site, site.source, '', filename)
            search_page.content = JSON.pretty_generate(search_data)
            search_page.data = {
              'layout' => 'none',
              'searchable' => false,
              'sitemap_include' => false,
              'permalink' => "#{lang}/search.json"
            }
            search_page.data['lang'] = lang unless lang == 'en'
            site.pages << search_page
          end
        end
      end

      puts "SearchIndexGenerator: Finished generating search index files."
    end

    private

    def generate_embedded_modules_search_index_data(site)
      puts "Generating embedded modules search index data..."

      # Generate embedded modules search indices for all languages
      embedded_modules_indices = {}
      excluded_names = ['CR.md', 'CR_RU.md', 'CONFIGURATION.md', 'CONFIGURATION_RU.md', 'CLUSTER_CONFIGURATION.md', 'CLUSTER_CONFIGURATION_RU.md']

      ['en', 'ru'].each do |lang|
        embedded_pages = site.pages.select { |page| page.data['sidebar'] == 'embedded-modules' && page.data['lang'] == lang }
        embedded_pages = embedded_pages.reject { |page| excluded_names.include?(page.name) }

        embedded_documents = embedded_pages.map do |page|
          keywords = []
          if page.data['module-kebab-name']
            keywords << page.data['module-kebab-name']
            keywords << page.data['module-snake-name'] if page.data['module-snake-name']
          end
          keywords << page.data['search'] if page.data['search'] && !page.data['search'].empty?

          {
            "title" => page.data['title'] || '',
            "url" => "/#{page.url.sub(/^\/(ru\/|en\/)/, '')}",
            "keywords" => keywords.join(', '),
            "module" => page.data['module-kebab-name'] || '',
            "moduletype" => "embedded",
            "summary" => (page.data['summary'] || page.data['description'] || '').strip,
            "content" => normalize_search_content(page.content || '')
          }
        end

        embedded_modules_indices[lang] = {
          "documents" => embedded_documents,
          "parameters" => []
        }
      end

      # Generate parameters for embedded modules (same for both languages)
      embedded_parameters = []
      if site.data['search'] && site.data['search']['searchItems']
        ['en', 'ru'].each do |lang|
          if site.data['search']['searchItems'][lang]
            site.data['search']['searchItems'][lang].each do |item|
              param = {
                "name" => item['name'] || '',
                "module" => item['module'] || '',
                "moduletype" => "embedded",
                "url" => item['url'] || '',
                "resName" => item['resourceName'] || '',
                "path" => item['pathString'] || '',
                "content" => normalize_search_content(item['content'] || '')
              }

              param["isResource"] = "true" if item['isResource']
              param["deprecated"] = "true" if item['deprecated']

              keywords = []
              keywords << item['search'] if item['search'] && !item['search'].empty?
              param["keywords"] = keywords.join(', ') if !keywords.empty?

              embedded_parameters << param
            end
          end
        end
      end

      # Add parameters to all embedded modules indices
      embedded_modules_indices.each do |lang, index|
        index["parameters"] = embedded_parameters
      end

      site.data['search_embedded_modules_indices'] = embedded_modules_indices

      puts "Embedded modules search index data generation completed."
    end

    def generate_search_index_data(site)
      puts "Generating search index data..."

      # Generate search indices for all languages
      search_indices = {}
      ['en', 'ru'].each do |lang|
        search_indices[lang] = generate_search_index(site, lang)
      end
      site.data['search_indices'] = search_indices

      # Generate embedded modules search indices for all languages
      embedded_modules_indices = {}
      excluded_names = ['CR.md', 'CR_RU.md', 'CONFIGURATION.md', 'CONFIGURATION_RU.md', 'CLUSTER_CONFIGURATION.md', 'CLUSTER_CONFIGURATION_RU.md']

      ['en', 'ru'].each do |lang|
        embedded_pages = site.pages.select { |page| page.data['sidebar'] == 'embedded-modules' && page.data['lang'] == lang }
        embedded_pages = embedded_pages.reject { |page| excluded_names.include?(page.name) }

        embedded_documents = embedded_pages.map do |page|
          keywords = []
          if page.data['module-kebab-name']
            keywords << page.data['module-kebab-name']
            keywords << page.data['module-snake-name'] if page.data['module-snake-name']
          end
          keywords << page.data['search'] if page.data['search'] && !page.data['search'].empty?

          {
            "title" => page.data['title'] || '',
            "url" => "/#{page.url.sub(/^\/(ru\/|en\/)/, '')}",
            "keywords" => keywords.join(', '),
            "module" => page.data['module-kebab-name'] || '',
            "moduletype" => "embedded",
            "summary" => (page.data['summary'] || page.data['description'] || '').strip,
            "content" => normalize_search_content(page.content || '')
          }
        end

        embedded_modules_indices[lang] = {
          "documents" => embedded_documents,
          "parameters" => []
        }
      end

      # Generate parameters for embedded modules (same for both languages)
      embedded_parameters = []
      if site.data['search'] && site.data['search']['searchItems']
        ['en', 'ru'].each do |lang|
          if site.data['search']['searchItems'][lang]
            site.data['search']['searchItems'][lang].each do |item|
              param = {
                "name" => item['name'] || '',
                "module" => item['module'] || '',
                "moduletype" => "embedded",
                "url" => item['url'] || '',
                "resName" => item['resourceName'] || '',
                "path" => item['pathString'] || '',
                "content" => normalize_search_content(item['content'] || '')
              }

              param["isResource"] = "true" if item['isResource']
              param["deprecated"] = "true" if item['deprecated']

              keywords = []
              keywords << item['search'] if item['search'] && !item['search'].empty?
              param["keywords"] = keywords.join(', ') if !keywords.empty?

              embedded_parameters << param
            end
          end
        end
      end

      # Add parameters to all embedded modules indices
      embedded_modules_indices.each do |lang, index|
        index["parameters"] = embedded_parameters
      end

      site.data['search_embedded_modules_indices'] = embedded_modules_indices

      puts "Search index data generation completed."
    end

    def generate_search_index(site, lang = nil)
      puts "Generating search index for language: #{lang || 'all'}"

      # Get pages for the specific language or all pages
      searched_pages = if lang
        site.pages.select { |page| page.data['searchable'] == true && page.data['lang'] == lang }
      else
        site.pages.select { |page| page.data['searchable'] == true }
      end

      # Filter out specific page types
      excluded_names = ['CR.md', 'CR_RU.md', 'CONFIGURATION.md', 'CONFIGURATION_RU.md', 'CLUSTER_CONFIGURATION.md', 'CLUSTER_CONFIGURATION_RU.md']
      searched_pages = searched_pages.reject { |page| excluded_names.include?(page.name) }

      # Generate documents section
      documents = searched_pages.map do |page|
        {
          "title" => page.data['title'] || '',
          "url" => page.url.sub(/^\/(ru\/|en\/)/, ''),
          "keywords" => page.data['search'] || [],
          "content" => normalize_search_content(page.content || '')
        }
      end

      # Generate parameters section from site.data.search.searchItems
      parameters = []
      if site.data['search'] && site.data['search']['searchItems']
        search_items = if lang && site.data['search']['searchItems'][lang]
          site.data['search']['searchItems'][lang]
        elsif !lang
          site.data['search']['searchItems'].values.flatten
        else
          []
        end

        parameters = search_items.map do |item|
          param = {
            "name" => item['name'] || '',
            "module" => item['module'] || 'global',
            "moduletype" => "embedded",
            "url" => item['url'] || '',
            "resName" => item['resourceName'] || '',
            "path" => item['pathString'] || '',
            "content" => normalize_search_content(item['content'] || '')
          }

          param["isResource"] = "true" if item['isResource']
          param["deprecated"] = "true" if item['deprecated']

          # Handle keywords
          keywords = []
          keywords << item['search'] if item['search'] && !item['search'].empty?
          param["keywords"] = keywords.join(', ') if !keywords.empty?

          param
        end
      end

      # Create the search index structure
      search_index = {
        "documents" => documents,
        "parameters" => parameters
      }

      search_index
    end

    def normalize_search_content(text)
      return '' if text.nil?

      # Apply the same transformations as normalizeSearchContent filter
      content = text.to_s

      # Remove HTML blocks
      content = content.gsub(/<script.*?<\/script>/m, ' ')
      content = content.gsub(/<!--.*?-->/m, ' ')
      content = content.gsub(/<style.*?<\/style>/m, ' ')

      # Remove HTML tags
      content = content.gsub(/<.*?>/m, ' ')

      # Remove markdown tables
      content = content.gsub(/\|\s*[:+\-= ]+\s*\|/, ' ')
      content = content.gsub(/[:+\-= ]{4,}/, ' ')
      content = content.gsub(/\|\|+/, ' ')
      # Remove complete markdown table rows (lines starting and ending with |)
      # Tables should be removed even if they contain inline code
      content = content.gsub(/^\|.*\|$/m, ' ')

      # Remove Liquid tags
      content = content.gsub(/\{\{.*?\}\}/m, ' ')
      content = content.gsub(/\{%.*?%\}/m, ' ')

      # Remove markdown code blocks
      content = content.gsub(/```[\s\S]*?```/m, ' ')
      content = content.gsub(/~~~[\s\S]*?~~~/m, ' ')
      content = content.gsub(/^```[\s\S]*?^```/m, ' ')
      content = content.gsub(/^~~~[\s\S]*?^~~~/m, ' ')

      # Remove shell blocks
      content = content.gsub(/<<\s*EOF[\s\S]*?^EOF/m, ' ')

      # Remove HTML div blocks
      content = content.gsub(/<div[^>]*markdown="0"[^>]*>[\s\S]*?<\/div>/m, ' ')

      # Remove specific inline code patterns (d8 and kubectl commands)
      content = content.gsub(/`d8 [^`]*`/, ' ')
      content = content.gsub(/`kubectl [^`]*`/, ' ')

      # Convert remaining inline code to plain text
      content = content.gsub(/`([^`]+)`/, '\1')

      # Remove unicode symbols
      content = content.gsub(/[\u{1F600}-\u{1F64F}\u{1F300}-\u{1F5FF}\u{1F680}-\u{1F6FF}\u{1F1E0}-\u{1F1FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}\u{1F900}-\u{1F9FF}\u{1FA70}-\u{1FAFF}\u{2000}-\u{206F}\u{2070}-\u{209F}\u{20A0}-\u{20CF}\u{2100}-\u{214F}\u{2190}-\u{21FF}\u{2200}-\u{22FF}\u{2300}-\u{23FF}\u{2400}-\u{243F}\u{2460}-\u{24FF}\u{25A0}-\u{25FF}\u{2B00}-\u{2BFF}\u{FE00}-\u{FE0F}\u{1F018}-\u{1F0F5}\u{1F200}-\u{1F2FF}]/u, ' ')

      # Normalize whitespace
      content = content.gsub(/\n/, ' ')
      content = content.gsub(/\s\s+/, ' ')
      content.strip
    end
  end
end
