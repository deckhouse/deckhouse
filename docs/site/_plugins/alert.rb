module Jekyll
  module Alert
    class AlertTag < Liquid::Block
      @@DEFAULTS = {
          :level => 'info',
          :class => 'docs__information',
          :active => true,
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
            if param[0].to_sym == 'active' then
              config[param[0].to_sym] = false if "#{param[1]}".downcase != 'true'
            else
              config[param[0].to_sym] = param[1]
            end
          end
          override_config(config)
        end
      end

      def render(context)
        content = super

        rendered_content = Jekyll::Converters::Markdown::KramdownParser.new(Jekyll.configuration()).convert(content)

        id = @config[:id] ? %Q(id="#{@config[:id]}") : ""
        %Q(<div markdown="0" #{id} class="#{@config[:class]} #{"active" if @config[:active]} #{@config[:level]}">#{rendered_content}</div>)

      end

      private

      def override_config(config)
        config.each{ |key,value| @config[key] = value }
      end

    end
  end
end

Liquid::Template.register_tag('alert', Jekyll::Alert::AlertTag)
