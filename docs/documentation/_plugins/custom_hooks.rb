require 'json'

def doc_links_for_module(moduleName)
    data = {
      'overview' => {
          'internal' => {
            'en' => "/en/modules/%s/" % moduleName,
            'ru' => "/ru/modules/%s/" % moduleName
          },
          'external' => {
            'en' => "/modules/%s/" % moduleName,
            'ru' => "/modules/%s/" % moduleName
          }
        },
      'crds' => []
    }
end

# Inserts the module-editions.liquid block into the module pages content.
# The block is inserted at the beginning of the page's content if the page content is not empty.
def insert_module_edition_block(page)
    additional_content = "\n{% include module-editions.liquid %}\n\n"

    page.content.prepend(additional_content) if page.content
end

# Inserts a block with the list of module web interfaces into the module pages content.
def insert_module_webiface_block(page)
    additional_content = "\n{% include module-webiface-notice.liquid %}\n\n"

    page.content.prepend(additional_content) if page.content
end

def find_in_entries(entries, item_name, item_value)
  entries.each do |entry|
    next if ! entry.is_a?(Hash)
    if entry.is_a?(Hash) and ( entry[item_name] == item_value or entry['title']['en'].downcase == item_value )
      return entry
    end
    if entry['folders']
      result = find_in_entries(entry['folders'], item_name, item_value)
      return result if result
    end
    if entry['entries']
      result = find_in_entries(entry['entries'], item_name, item_value)
      return result if result
    end
  end
  nil
end

# Inserts a block with the warnings about module stage.
def insert_module_stage_block(sidebar, page)
    return if !sidebar
    moduleData = find_in_entries(sidebar, 'moduleName', page.data['module-kebab-name'])
    if moduleData and moduleData['featureStatus']
        additional_content = "\n{% include warning-version.liquid stage=\"#{moduleData['featureStatus']}\" %}\n\n"
        page.content.prepend(additional_content)
    end
end

# Generate search index JSON files using pre_render hook
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

# Normalize search content using the same logic as the custom filter
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
  
  # Remove specific inline code patterns (d8 k and kubectl commands)
  content = content.gsub(/`d8 k[^`]*`/, ' ')
  content = content.gsub(/`kubectl[^`]*`/, ' ')
  
  # Convert remaining inline code to plain text
  content = content.gsub(/`([^`]+)`/, '\1')
  
  # Remove unicode symbols
  content = content.gsub(/[\u{1F600}-\u{1F64F}\u{1F300}-\u{1F5FF}\u{1F680}-\u{1F6FF}\u{1F1E0}-\u{1F1FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}\u{1F900}-\u{1F9FF}\u{1FA70}-\u{1FAFF}\u{2000}-\u{206F}\u{2070}-\u{209F}\u{20A0}-\u{20CF}\u{2100}-\u{214F}\u{2190}-\u{21FF}\u{2200}-\u{22FF}\u{2300}-\u{23FF}\u{2400}-\u{243F}\u{2460}-\u{24FF}\u{25A0}-\u{25FF}\u{2B00}-\u{2BFF}\u{FE00}-\u{FE0F}\u{1F018}-\u{1F0F5}\u{1F200}-\u{1F2FF}]/u, ' ')
  
  # Normalize whitespace
  content = content.gsub(/\n/, ' ')
  content = content.gsub(/\s\s+/, ' ')
  content.strip
end

##
Jekyll::Hooks.register :site, :pre_render do |site|
  bundlesByModule = Hash.new()
  bundlesModules = Hash.new()
  bundleNames = []

  puts "Custom hook: pre_render"

  site.data['bundles'] = Hash.new() if ! site.data.has_key?('bundles')
  site.data['bundles']['raw'] = Hash.new() if ! site.data['bundles'].has_key?('raw')

  site.data['bundles']['raw'].each do |revision, revisionData|
    revisionData.each do |key, val|
      bundleName = key.delete_prefix('values-').capitalize

      if ! bundleNames.include?(bundleName) then  bundleNames << bundleName end

      if ! val then next end

      val.each do |_moduleName, _status|
        moduleName = _moduleName.to_s.
                           delete_suffix('Enabled').
                           gsub(/([A-Z]+)([A-Z][a-z])/,'\1-\2').
                           gsub(/([a-z\d])([A-Z])/,'\1-\2').downcase
        status = _status.to_s.downcase

        if ! bundlesByModule.has_key?(moduleName) then
          bundlesByModule[moduleName] = Hash.new()
        end
        bundlesByModule[moduleName][bundleName] = status
        if ! bundlesModules[bundleName] then bundlesModules[bundleName] = [] end
        if status == "true" then bundlesModules[bundleName] << moduleName end
      end
    end
  end

  site.data['bundles']['byModule'] = bundlesByModule
  site.data['bundles']['bundleNames'] = bundleNames.sort
  site.data['bundles']['bundleModules'] = bundlesModules

  site.data['modules'] = Hash.new() if ! site.data.has_key?('modules')
  site.data['modules']['all'] = Hash.new() if ! site.data['modules'].has_key?('all')

  _editionsFullList = site.data['modules']['editions-weight'].keys
  # Automatically fill editions, except for CSE, since their CSE needs to be specified explicitly.
  _editionsToFillWith = _editionsFullList.reject { |key| key.start_with?("cse")  }
  site.data['modules']['all'].each do |moduleName, moduleData|
    editions = []
    if moduleData.has_key?("editionMinimumAvailable") then
      _index = _editionsToFillWith.find_index(moduleData['editionMinimumAvailable'])
      editions = _editionsToFillWith.slice(_index, _editionsToFillWith.length())
    else
      editions = editions | moduleData['editions'] if moduleData.has_key?("editions")
    end
    site.data['editions'].each do |edition, editionData|
      editions = editions | [edition] if editionData.has_key?("includeModules") && editionData['includeModules'].include?(moduleName)
      editions.delete(edition) if editionData.has_key?("excludeModules") && editionData['excludeModules'].include?(moduleName)
    end
    editions = editions | moduleData['editionFullyAvailable'] if moduleData.has_key?("editionFullyAvailable")
    editions = editions | moduleData['editionsWithRestrictions'] if moduleData.has_key?("editionsWithRestrictions")
    puts "Module #{moduleName} editions: #{editions}"
    site.data['modules']['all'][moduleName]['editions'] = editions

    site.data['modules']['all'][moduleName]['docs'] = doc_links_for_module(moduleName)
  end

  # Exclude custom resource and module setting files from the search index by setting the 'searchable' parameter to false.
  # Add module name in kebab case and snake case to metadata of module pages.
  # Add module name in kebab case and snake case to search keywords.

  pageAllowedSuffixes = [
    'CONFIGURATION.md', 'CONFIGURATION_RU.md',
    'CR_RU.md', 'CR.md',
    'EXAMPLES_RU.md', 'EXAMPLES.md',
    'USAGE_RU.md', 'USAGE.md',
    'FAQ_RU.md', 'FAQ.md'
  ]

  # Set the following data for each module page:
  # - module-kebab-name: module name in kebab case
  # - module-snake-name: module name in snake case
  site.pages.each do |page|
    # if page.url.match?(%r{/modules/([0-9]+-)?[^/]+/$}) || (page.name && pageAllowedSuffixes.any? { |suffix| page.name.end_with?(suffix) })
    #if page.dir.match?(%r{/modules(_en|_ru)/([0-9]+-)?[^/]+/docs/$}) && (page.name && pageAllowedSuffixes.any? { |suffix| page.name.end_with?(suffix) })
    if page.dir.match?(%r{(/en|/ru)/modules/([0-9]+-)?[^/]+/$})
      moduleKebabCase = page.url.sub(%r{(.*)?/modules/([0-9]+-)?([^/]+)/.*$},'\3')
      moduleSnakeCase = moduleKebabCase.gsub(/-[a-z]/,&:upcase).gsub(/-/,'')
      page.data['module-kebab-name'] = moduleKebabCase
      page.data['module-snake-name'] = moduleSnakeCase
      page.data['sidebar'] = 'embedded-modules'
      if  page.name.match?(/CONFIGURATION(\.ru|_RU)?\.md$/) then
        page.data['legacy-enabled-commands'] = %Q(#{moduleSnakeCase}Enabled)
      else
        page.data['module-index-page'] = true
      end
    end

    if page.data['webIfaces']
      insert_module_webiface_block(page)
    end

    if page.data['module-kebab-name'] and !page.name.match?(/CR(\.ru|_RU)?\.md$/)
      # TODO Fix it
      # insert_module_stage_block(site.data['sidebars'][page.data['sidebar']]['entries'], page)

      if page.name.match?(/^README(\.ru|_RU)?\.md$/i) ||
         page.name.match?(/^CONFIGURATION(\.ru|_RU)?\.md$/i)
        insert_module_edition_block(page)
      end
    end

    next if page.name && ! ( page.name.end_with?('CR.md') or page.name.end_with?('CR_RU.md') or page.name.end_with?('CONFIGURATION.md') or page.name.end_with?('CONFIGURATION_RU.md') )
    next if page['force_searchable'] == true
    page.data['searchable'] = false
  end

  # Generate search index JSON files
  puts "Generating search index files..."
  
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
  
  puts "Search index generation completed."
end
