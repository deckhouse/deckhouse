require 'json'

module Jekyll
  class SearchIndexGenerator < Generator
    safe true
    priority :lowest

    def generate(site)
      puts "SearchIndexGenerator: Generating search index files..."

      # Check if embedded_modules mode is enabled
      embedded_modules_mode = site.config['embedded_modules'] == true
      puts "SearchIndexGenerator: Embedded modules mode enabled: #{embedded_modules_mode}"

      if embedded_modules_mode
        puts "Embedded modules mode enabled - generating only embedded modules search indices"
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

  end
end
