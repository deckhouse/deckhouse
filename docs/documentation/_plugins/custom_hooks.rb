require 'json'

def parse_module_data(input, site)
    if input.has_key?("featureStatus")
       featureStatus = input["featureStatus"]
       if input.has_key?("moduleName")
         moduleName = input["moduleName"]
       elsif input["title"].is_a?(Hash) && input["title"].has_key?('en')
         moduleName = input["title"]['en']
       else
         moduleName = input["title"]
       end
       if ! site.data["modulesFeatureStatus"]
          site.data["modulesFeatureStatus"] = {}
       end
       site.data["modulesFeatureStatus"][moduleName] = featureStatus
    else
      if input.has_key?("folders")
        input["folders"].each do |item|
          parse_module_data(item, site)
        end
      end
    end
end

Jekyll::Hooks.register :site, :post_read do |site|
  bundlesByModule = Hash.new()
  bundlesModules = Hash.new()
  bundleNames = []

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

  # Fill site.data.modulesFeatureStatus
  parse_module_data(site.data["sidebars"]["main"]["entries"], site)

  # Exclude custom resource and module setting files from the search index by setting the 'searchable' parameter to false.
  # Add module name in kebab case and snake case to metadata of module pages.
  # Add module name in kebab case and snake case to search keywords.

  pageSuffixes = [
    'CONFIGURATION.md', 'CONFIGURATION_RU.md',
    'CR_RU.md', 'CR.md',
    'EXAMPLES_RU.md', 'EXAMPLES.md',
    'USAGE_RU.md', 'USAGE.md',
    'FAQ_RU.md', 'FAQ.md'
  ]

  site.pages.each do |page|
    if page.url.match?(%r{/modules/([0-9]+-)?[^/]+/$}) || pageSuffixes.any? { |suffix| page.name.end_with?(suffix) }
    then
      moduleKebabCase = page.url.sub(%r{(.*)?/modules/([0-9]+-)?([^/]+)/.*$},'\3')
      moduleSnakeCase = moduleKebabCase.gsub(/-[a-z]/,&:upcase).gsub(/-/,'')
      page.data['module-kebab-name'] = moduleKebabCase
      page.data['module-snake-name'] = moduleSnakeCase
      if ( page.name == 'CONFIGURATION.md' or page.name == 'CONFIGURATION_RU.md' ) then
        page.data['legacy-enabled-commands'] = %Q(#{moduleSnakeCase}Enabled)
      else
        page.data['module-index-page'] = true
      end
    end
    next if ! ( page.name.end_with?('CR.md') or page.name.end_with?('CR_RU.md') or page.name.end_with?('CONFIGURATION.md') or page.name.end_with?('CONFIGURATION_RU.md') )
    next if page['force_searchable'] == true
    page.data['searchable'] = false
  end
end
