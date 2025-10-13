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
end
