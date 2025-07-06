require 'json'

# Generate breadcrumbs file for all the sidebars.

Jekyll::Hooks.register :site, :pre_render do |site|

  puts "Generating breadcrumbs..."

  site.data['breadcrumbs'] = {} if site.data['breadcrumbs'].nil?
  #breadcrumbsSite = site.data['breadcrumbs']

  site.data['sidebars'].each do |item|
    sidebarData = item[1]
    sidebarName = item[0]

    if sidebarData['entries']
      sidebarData['entries'].each do |entry|
        if entry.is_a?(Hash) && entry['folders'].is_a?(Array)
          #breadcrumbs = processSidebarItem([], entry)
          #breadcrumbsSite.merge(breadcrumbs) if breadcrumbs && !breadcrumbs.empty?
          site.data['breadcrumbs'] = site.data['breadcrumbs'].merge(processSidebarItem([], entry))
        end
      end
    end
  end

  #puts "[DEBUG]Breadcrumbs..."
  #puts breadcrumbsSite
  #File.open('breadcrumbs2.yaml', 'w') { |f| f.write(breadcrumbsSite.to_yaml) }
  #raise
end

def processSidebarItem(parents, sidebarItem)

    return {} if sidebarItem.nil? || sidebarItem.empty?

    breadcrumbs = {}

    if sidebarItem['folders'] && sidebarItem['folders'].is_a?(Array)
      sidebarItem['folders'].each do |folder|
          #breadcrumbsFolders = processSidebarItem(parents + [{'title' => sidebarItem['title']}], folder)
#          breadcrumbs + breadcrumbsFolders if breadcrumbsFolders && !breadcrumbsFolders.empty?
          breadcrumbs = breadcrumbs.merge(processSidebarItem(parents + [{'title' => sidebarItem['title']}], folder))
          #puts '####################'
          #puts breadcrumbsFolders
          #puts 'breadcrumbs-----------'
          #puts breadcrumbs
      end
    else
      if sidebarItem['url']
        #section_url = sidebarItem['url'].split('/')[0...-1].join('/')
        section_url = sidebarItem['url']
        breadcrumbs[section_url] = parents
#        breadcrumbs << {
#          section_url => {
#            'title' => {
#              'en' => parents['en'],
#              'ru' => parents['ru'],
#            }
#          }
#        }
#              'title' => {
#                'en' => folder['title']['en'],
#                'ru' => folder['title']['ru']
#              }
      end
    end
    #raise "DEBUG #{breadcrumbs}"
    breadcrumbs
end
