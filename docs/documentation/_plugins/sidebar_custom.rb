require 'shellwords'
require_relative "utils"

module Jekyll
  class SidebarCustomTag < Liquid::Tag

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
      @config[:sidebar] = context.registers[:site].data['sidebars'][context.registers[:page]['sidebar']]

      return if @config[:sidebar].nil?

      @config[:sidebar]['entries'].each do |entry|
        result.push(sidebar_entry(entry, parameters))
      end
      result.join("\n")
    end

    def sidebar_entry(entry, parameters)

      lang = @context.registers[:page]['lang']

      return if !entry || ! entry.dig('title',lang) || entry['draft'] == true

      sidebar_validate_item(entry)

      result = []
      not_avail_in_this_edition = false
      avail_in_commercial_editions_only = false
      if parameters.has_key?('not_avail_in_this_edition') && parameters['not_avail_in_this_edition'] == true
        not_avail_in_this_edition = true
      end
      doc_edition = @context.registers[:site].config['d8Revision'].downcase
      site_mode = @context.registers[:site].config['mode'] ? @context.registers[:site].config['mode'].downcase : ''

      if entry.has_key?('moduleName') && entry['moduleName'] != ''
          module_name = entry['moduleName']
      else
          module_name = entry.dig('title',lang)
      end

      if @context.registers[:site].data.dig('modules', 'all', module_name, 'editions')
          if ! @context.registers[:site].data.dig('modules', 'all', module_name, 'editions').include?(doc_edition)
             not_avail_in_this_edition = true
          end
          if ! @context.registers[:site].data.dig('modules', 'all', module_name, 'editions').include?('ce')
            avail_in_commercial_editions_only = true
          end
      end

      entry_with_lang = "/%s%s" % [lang, entry['url']]
      page_url = @context.registers[:page]['url'].sub(/\/index.html?$/, '/')
      sidebar_group_page = @context.registers[:page]['sidebar_group_page']

      if not_avail_in_this_edition && site_mode == 'module' and !entry.has_key?('external_url')
           external_url = "%s%s%s" % [ @context.registers[:site].config['urls'][lang], @context.registers[:site].config['canonical_url_prefix_documentation'], entry['url'] ]
      end

      if entry.dig('folders') && entry['folders'].size > 0
         result.push(%Q(<li class='#{ parameters['folder_entry_class']}'>
            <a href='#'>
              <span class='sidebar__submenu-title'>#{entry.dig('title',lang)}</span>

              <span class='sidebar__badge--container'>))
         if (site_mode != 'module' && avail_in_commercial_editions_only) ||
            (doc_edition == 'ce' && avail_in_commercial_editions_only) ||
            (site_mode == 'module' && not_avail_in_this_edition)
             result.push(%Q(
             <span class="sidebar__badge_v2 sidebar__badge_commercial"
                   title="#{ if site_mode == 'module' and not_avail_in_this_edition
                                @context.registers[:site].data['i18n']['features']['notAvailableInThisEdition'][lang].sub(/\.$/, '')
                             else
                                @context.registers[:site].data['i18n']['features']['commercial'][lang].sub(/\.$/, '')
                             end}"
             >#{ @context.registers[:site].data['i18n']['features']['currency_sign'][lang] }</span>))
         end

         if entry.has_key?('featureStatus')
           result.push(%Q(
               <span class='sidebar__badge_v2'
                     title="#{ @context.registers[:site].data['i18n']['features']["%s_long" % entry['featureStatus']][lang].gsub(/<\/?[^>]*>/, "").lstrip.sub(/\.$/, '')}">
                   #{case entry['featureStatus']
                       when "preview"
                           "Preview"
                       when "experimental"
                           "Exp"
                       when "deprecated"
                           "Dep"
                       when "temporaryDeprecated"
                           "Temporary deprecated"
                       when "proprietaryOkmeter"
                           "Prop"
                     end}))
           result.push('</span>')
         end

         result.push(%q(
           </span>
           </a>
           <ul class='sidebar__submenu'>))
         entry['folders'].each do |sub_entry|
            result.push(sidebar_entry(sub_entry, {"folder_entry_class" => "sidebar__submenu-item sidebar__submenu-item_parent", "item_entry_class" => "sidebar__submenu-item", "not_avail_in_this_edition" => not_avail_in_this_edition}))
         end
         result.push(%q(
           </ul>
         </li>))
      elsif entry.has_key?('external_url')
        result.push("<li class='#{ parameters['item_entry_class']}'><a href='#{ entry['external_url'] }' target='_blank'>#{entry.dig('title', lang)} ↗</a></li>")
      elsif !external_url.nil? && external_url.size > 0
        result.push("<li class='#{ parameters['item_entry_class']}'><a href='#{ external_url }' target='_blank'>#{entry.dig('title', lang)} ↗</a></li>")
      elsif page_url == entry['url'] or page_url == entry_with_lang or sidebar_group_page == entry['url']
        result.push("<li class='#{ parameters['item_entry_class']} active'><a href='#{ getTrueRelativeUrl(entry['url']) }'>#{entry.dig('title', lang)}</a></li>")
      else
        if @context.registers[:page]['url'] == '404.md'
          result.push(%Q(<li class='#{ parameters['item_entry_class']}'><a data-proofer-ignore href='#{ @context.registers[:site].config['canonical_url_prefix_documentation'] + getTrueRelativeUrl(entry['url']) }'>#{entry.dig('title', lang)}</a></li>))
        else
          result.push(%Q(<li class='#{ parameters['item_entry_class']}'><a href='#{ getTrueRelativeUrl(entry['url']) }'>#{entry.dig('title', lang)}</a></li>))
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

Liquid::Template.register_tag('sidebar_custom', Jekyll::SidebarCustomTag)
