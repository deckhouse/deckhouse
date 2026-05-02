require_relative "utils"

module Jekyll
  module Offtopic
    class OfftopicTag < Liquid::Block
      include JekyllLiquidBlockUtils

      @@DEFAULTS = {
          :title => 'Подробности',
      }

      def self.DEFAULTS
        return @@DEFAULTS
      end

      def initialize(tag_name, markup, tokens)
        super

        @config = {}
        override_config(@@DEFAULTS)

        params = markup.scan /([a-z]+)\=\"(.+?)\"/
        if params.size > 0
          config = {}
          params.each do |param|
            config[param[0].to_sym] = param[1]
          end
          override_config(config)
        end
      end

      def override_config(config)
        config.each{ |key,value| @config[key] = value }
      end

      def render(context)
        content = dedent(super)

        site = context.registers[:site]
        @converter = site.find_converter_instance(::Jekyll::Converters::Markdown)
        rendered_content = collapse_inter_block_newlines(@converter.convert(content))

        rendered_content = rendered_content.gsub(/(<pre\b[^>]*>)(.*?)(<\/pre>)/m) do
          pre_open = $1
          pre_body = $2
          pre_close = $3
          "#{pre_open}#{pre_body.gsub("\n", '&#10;')}#{pre_close}"
        end

        %Q(<div markdown="0" class="details"><p class="details__lnk"><a href="javascript:void(0)" class="details__summary">#{@config[:title]}</a></p><div class="details__content"><div class="expand">#{rendered_content}</div></div></div>)
      end
    end
  end
end

Liquid::Template.register_tag('offtopic', Jekyll::Offtopic::OfftopicTag)
