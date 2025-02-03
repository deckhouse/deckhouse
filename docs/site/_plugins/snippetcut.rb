require "liquid"

module Jekyll
  module SnippetCut
    class SnippetCutTag < Liquid::Block
      @@DEFAULTS = {
          :name => '',
          :url => '#',
          :limited => false
      }

      def self.DEFAULTS
        return @@DEFAULTS
      end

      def initialize(tag_name, raw_markup, tokens)
        super
        @raw_markup = raw_markup
        @config = {}
      end

      def render(context)
        site = context.registers[:site]
        @converter = site.find_converter_instance(::Jekyll::Converters::Markdown)

        @markup = Liquid::Template
          .parse(@raw_markup)
          .render(context)
          .gsub(%r!\\\{\\\{|\\\{\\%!, '\{\{' => "{{", '\{\%' => "{%")
          .strip

        override_config(@@DEFAULTS)

        params = @markup.scan /([a-z]+)\=\"(.+?)\"/
        if params.size > 0
          config = {}
          params.each do |param|
            config[param[0].to_sym] = param[1]
          end
          override_config(config)
        end

        content = super
        rendered_content = @converter.convert(content)
        #         <div class="snippetcut#{@config[:limited] ? ' snippetcut_limited' : ''}" data-snippetcut>
        #             <div class="snippetcut__title">#{if (@config[:url]!='#') then "<a href=\""+@config[:url]+"\" target=\"_blank\"
        #                                                                               class=\"snippetcut__title-name\"
        #                                                                               data-snippetcut-name>" else "<span
        #                     class=\"snippetcut__title-name-text\" data-snippetcut-name>" end}#{@config[:name]}#{if (@config[:url]!='#') then "</a>"
        #                 else "</span>" end}#{if (@config[:name] != '') then "<a href='javascript:void(0)' class='snippetcut__title-btn'
        #                                                                         data-snippetcut-btn-name-#{context['page']['lang']}>#{context['site']['data']['i18n']['copy_filename'][context['page']['lang']]}</a>
        #                 " end}<a href="javascript:void(0)" class="snippetcut__title-btn" data-snippetcut-btn-text-#{context['page']['lang']}>#{context['site']['data']['i18n']['copy_content'][context['page']['lang']]}</a>
        #             </div>
        #             <div class="highlight">#{remove_excessive_newlines(rendered_content)}</div>
        #             <div #{if (@config[:selector] != '') then "#{@config[:selector]}" end } class="snippetcut__raw" data-snippetcut-text>
        #                 #{CGI::escapeHTML(remove_excessive_newlines(content.strip.gsub(%r!^\s*```[a-zA-Z0-9]*!,'')))}
        #             </div>
        #         </div>

        %Q(<div class="snippetcut#{@config[:limited] ? ' snippetcut_limited' : ''}" data-snippetcut><div class="snippetcut__title">#{if (@config[:url]!='#') then "<a href=\""+@config[:url]+"\" target=\"_blank\" class=\"snippetcut__title-name\" data-snippetcut-name>" else "<span class=\"snippetcut__title-name-text\" data-snippetcut-name>" end}#{@config[:name]}#{if (@config[:url]!='#') then "</a>" else "</span>" end}#{if (@config[:name] != '') then "<a href='javascript:void(0)' class='snippetcut__title-btn' data-snippetcut-btn-name-#{context['page']['lang']}>#{context['site']['data']['i18n']['copy_filename'][context['page']['lang']]}</a>" end}<a href="javascript:void(0)" class="snippetcut__title-btn" data-snippetcut-btn-text-#{context['page']['lang']}>#{context['site']['data']['i18n']['copy_content'][context['page']['lang']]}</a></div><div class="highlight">#{remove_excessive_newlines(rendered_content)}</div><div #{ if (@config[:selector] != '') then "#{@config[:selector]}" end } class="snippetcut__raw" data-snippetcut-text>#{CGI::escapeHTML(remove_excessive_newlines(content.strip.gsub(%r!^\s*```[a-zA-Z0-9]*!,'')))}</div></div>)
      end

      private

      def override_config(config)
        config.each{ |key,value| @config[key] = value }
      end

      def remove_excessive_newlines(text)
        return text.sub(/^(\s*\R)*/, "").rstrip()
      end
    end
  end
end

Liquid::Template.register_tag('snippetcut', Jekyll::SnippetCut::SnippetCutTag)
