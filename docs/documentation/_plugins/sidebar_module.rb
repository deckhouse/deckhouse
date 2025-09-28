require 'json'
require_relative "utils"

Jekyll::Hooks.register :site, :pre_render do |site|

  puts "Generating sidebar for embedded modules..."

  site.data['sidebars']['embedded-modules'] = {} if site.data['sidebars']['embedded-modules'].nil?

  site.pages.each do |page|
    if page.url.match?(%r{/modules/[^/]+/(.+)?$})
    then
      lang = page['lang'] || 'en'
      sidebarUrl = page.url.sub(%r{^/(ru|en)/},'/')
      moduleKebabCase = page.url.sub(%r{(.*)?/modules/([^/]+)/.*$},'\2')
      # puts "Processing page: #{page.name} with URL: #{page.url}. Module #{moduleKebabCase}. Sidebar URL: #{sidebarUrl}, weight: #{weight} (#{File.basename(sidebarUrl)})"
      # Initialize the sidebar entry for the module if it does not exist
      if site.data['sidebars']['embedded-modules'][moduleKebabCase].nil?
        site.data['sidebars']['embedded-modules'][moduleKebabCase] = {
           'title' => {
             lang => "#{site.data['i18n']['common']['module'][lang].capitalize} #{moduleKebabCase}"
           },
           'root' => true,
           'folders' => [],
        }
      end

      if site.data['sidebars']['embedded-modules'][moduleKebabCase]['title'][lang].nil?
         site.data['sidebars']['embedded-modules'][moduleKebabCase]['title'][lang] = "#{site.data['i18n']['common']['module'][lang].capitalize} #{moduleKebabCase}"
      end

      pageSidebarEntry = site.data['sidebars']['embedded-modules'][moduleKebabCase]['folders'].find { |p| p['url'] == sidebarUrl }

      if pageSidebarEntry.nil?
        # If the entry doesn't exist, create it

        # Determine the weight for the sidebar entry according to the data.modules.sidebar.weight
        if sidebarUrl.end_with?("/")
          weight = 1
        else
          weight = site.data['modules']['sidebar']['weight'][File.basename(sidebarUrl)] || 100
        end

        pageSidebarEntry = {
          'title' => {
            lang => page.data['title'] || page.name.sub(/\.md$/, '').gsub(/-/, ' ').capitalize
          },
          'url' => sidebarUrl,
          'moduleName' => moduleKebabCase,
          'moduleSnakeName' => moduleKebabCase.gsub(/-[a-z]/,&:upcase).gsub(/-/,''),
          'weight' => weight
        }
        site.data['sidebars']['embedded-modules'][moduleKebabCase]['folders'].push(pageSidebarEntry)
      else
        pageSidebarEntry['title'][lang] = page.data['title'] || page.name.sub(/\.md$/, '').gsub(/-/, ' ').capitalize
      end
    end

  end

  if site.data['sidebars']['embedded-modules'].empty?
    puts "Sidebar for embedded modules is empty (maybe it is not necessary here at all)."
  else
    puts "Sidebar for embedded modules has been generated."
  end
end

module Jekyll
  class SidebarModuleTag < Liquid::Tag

    def initialize(tag_name, markup, parse_context)
      # @type [String]
      @markup = markup.strip
      @config = {}
      @context = parse_context
      super
    end

    def render(context)
      result = []
      @context = context
      parameters = parse_parameters(context)
      moduleName = context.registers[:page]['module-kebab-name'] || context.registers[:page]['moduleName']
      @config[:sidebar] = context.registers[:site].data['sidebars'][context.registers[:page]['sidebar']]

      return if @config[:sidebar].nil?

      @config[:sidebar][moduleName]['folders'].sort_by! { |entry| entry['weight'] }.each do |entry|
        result.push(sidebar_entry(entry, parameters))
      end
      result.join("\n")
    end

    def sidebar_entry(entry, parameters)

      lang = @context.registers[:page]['lang']
      moduleName = entry['moduleName']
      page_url = @context.registers[:page]['url'].sub(%r{/index.html$}, '/').sub(%r{^/?(en/|/ru/)?modules/[^/]+/},'./')
      entry_url_without_module_path = entry['url'].sub(%r{^/?modules/[^/]+/},'./')

      if entry['url'].end_with?('/')
        sidebarItemTitle = @context.registers[:site].data['modules']['sidebar']['titles']['overview'][lang]
      else
        sidebarItemTitle = @context.registers[:site].data['modules']['sidebar']['titles'].dig(File.basename(entry['url']), lang) || entry.dig('title',lang)
      end

      return if !entry || ! sidebarItemTitle || ! moduleName || entry['draft'] == true

      sidebar_validate_item(entry)

      result = []
      not_avail_in_this_edition = false
      avail_in_commercial_editions_only = false
      if parameters.has_key?('not_avail_in_this_edition') && parameters['not_avail_in_this_edition'] == true
        not_avail_in_this_edition = true
      end
      doc_edition = @context.registers[:site].config['d8Revision'].downcase
      site_mode = @context.registers[:site].config['mode'] ? @context.registers[:site].config['mode'].downcase : ''

      if @context.registers[:site].data.dig('modules', 'all', moduleName, 'editions')
          if ! @context.registers[:site].data.dig('modules', 'all', moduleName, 'editions').include?(doc_edition)
             not_avail_in_this_edition = true
          end
          if ! @context.registers[:site].data.dig('modules', 'all', moduleName, 'editions').include?('ce')
            avail_in_commercial_editions_only = true
          end
      end

      # TODO Delete this (sidebar_group_page is not used in the module sidebar)
      # sidebar_group_page = @context.registers[:page]['sidebar_group_page']

      if not_avail_in_this_edition && site_mode == 'module' and !entry.has_key?('external_url')
           external_url = "%s%s%s" % [ @context.registers[:site].config['urls'][lang], @context.registers[:site].config['canonical_url_prefix_documentation'], entry['url'] ]
      end

      if entry.has_key?('external_url')
        result.push("<li class='#{ parameters['item_entry_class']}'><a href='#{ entry['external_url'] }' target='_blank'>#{sidebarItemTitle} ↗</a></li>")
      elsif !external_url.nil? && external_url.size > 0
        result.push("<li class='#{ parameters['item_entry_class']}'><a href='#{ external_url }' target='_blank'>#{sidebarItemTitle} ↗</a></li>")
      elsif page_url == entry_url_without_module_path
        #or sidebar_group_page == entry['url']
        result.push("<li class='#{ parameters['item_entry_class']} active'><a href='#{ entry_url_without_module_path }'>#{sidebarItemTitle}</a></li>")
      else
        if @context.registers[:page]['url'] == '404.md'
          # There is no sidebar on 404 page yet.
          result.push(%Q(<li class='#{ parameters['item_entry_class']}'><a data-proofer-ignore href='#{ @context.registers[:site].config['canonical_url_prefix_documentation'] + entry['url'] }'>#{sidebarItemTitle}</a></li>))
        else
          result.push(%Q(<li class='#{ parameters['item_entry_class']}'><a href='#{ entry_url_without_module_path }'>#{sidebarItemTitle}</a></li>))
        end
      end

      result.join("\n")
    end

    def parse_parameters(context)
      _parameters = Shellwords.split(@markup)
      result = {}

      _parameters.map do |a|
        k, *v = a.split(/\s*[=:]\s*/, 0)
        if v.size==1
          v = v[0]
        elsif v.size==0
          v = nil
        end
        result[k] = typecast(v)
      end
      result
    end

    def typecast v
      if v.is_a? Array
        return v.map{|item| typecast(item)}
      end
      if v=='true' or v.nil?
        v = true
      elsif v=='false'
        v = false
      elsif v=~/^-?\d*\.\d+$/
        v = v.to_f
      elsif v=~/^-?\d+$/
        v = v.to_i
      else
        v
      end
    end

  end
end

Liquid::Template.register_tag('sidebar_module', Jekyll::SidebarModuleTag)
