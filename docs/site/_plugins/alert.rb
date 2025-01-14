module Jekyll
  module Alert
    class AlertTag < Liquid::Block
      @@DEFAULTS = {
          :level => 'info',
          :tag => 'div',
          :class => 'alert__wrap',
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
        site = context.registers[:site]
        @converter = site.find_converter_instance(::Jekyll::Converters::Markdown)

        rendered_content = @converter.convert(content)

        id = @config[:id] ? %Q(id="#{@config[:id]}") : ""
        %Q(<#{@config[:tag]} markdown="0" #{id} class="#{@config[:level]} #{@config[:class]}">
          <svg class="alert__icon icon--#{@config[:level]}">
            <use xlink:href="/images/sprite.svg##{@config[:level]}-icon"></use>
          </svg>
          <div>#{rendered_content}</div>
        </#{@config[:tag]}>)

      end

      private

      def override_config(config)
        config.each{ |key,value| @config[key] = value }
      end

    end
  end
end

Liquid::Template.register_tag('alert', Jekyll::Alert::AlertTag)
