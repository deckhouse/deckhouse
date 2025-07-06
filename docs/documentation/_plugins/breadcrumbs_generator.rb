require 'json'

# Generate breadcrumbs file for all the sidebars.

Jekyll::Hooks.register :site, :pre_render do |site|

  puts "Generating breadcrumbs..."

  site.data['breadcrumbs'] = {} if site.data['breadcrumbs'].nil?

  site.data['sidebars'].each do |item|
    sidebarData = item[1]
    sidebarName = item[0]

    if sidebarData['entries']
      sidebarData['entries'].each do |entry|
        if entry.is_a?(Hash) && entry['folders'].is_a?(Array)
          site.data['breadcrumbs'] = site.data['breadcrumbs'].merge(processSidebarItem([], entry))
        end
      end
    end
  end
end

def processSidebarItem(parents, sidebarItem)

    return {} if sidebarItem.nil? || sidebarItem.empty?

    breadcrumbs = {}

    if sidebarItem['folders'] && sidebarItem['folders'].is_a?(Array)
      sidebarItem['folders'].each do |folder|
          breadcrumbs = breadcrumbs.merge(processSidebarItem(parents + [{'title' => sidebarItem['title']}], folder))
      end
    else
      if sidebarItem['url']
        section_url = sidebarItem['url']
        breadcrumbs[section_url] = parents
      end
    end
    breadcrumbs
end
