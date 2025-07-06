require 'json'

# Generate breadcrumbs file for all the sidebars.

Jekyll::Hooks.register :site, :pre_render do |site|

  puts "Generating breadcrumbs..."

  site.data['breadcrumbs'] = {} if site.data['breadcrumbs'].nil?

  site.data['sidebars'].each do |sidebarName, sidebarData|
    next unless sidebarData['entries']

    sidebarData['entries'].each do |entry|
      if entry.is_a?(Hash) && entry['folders'].is_a?(Array)
        site.data['breadcrumbs'].merge!(processSidebarItem([], entry))
      end
    end
  end
end

def processSidebarItem(parents, sidebarItem)

    return {} if sidebarItem.nil? || sidebarItem.empty?

    breadcrumbs = {}

    if sidebarItem['folders'].is_a?(Array)
      sidebarItem['folders'].each do |folder|
          breadcrumbs.merge!(processSidebarItem(parents + [{'title' => sidebarItem['title']}], folder))
      end
    elsif sidebarItem['url'].to_s.strip != ''
      breadcrumbs[sidebarItem['url']] = parents
    end
    breadcrumbs
end
