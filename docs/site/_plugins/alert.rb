module Jekyll
  module Alert
    class AlertTag < Liquid::Block
      @@DEFAULTS = {
        level:  'info',
        tag:    'div',
        class:  'alert__wrap',
        active: true
      }

      CLASS_MAP = {
        'info'     => 'notice notice--note',
        'warning'  => 'notice notice--warning',
        'danger'   => 'notice notice--caution',
        'note'     => 'notice notice--note',
        'tip'      => 'notice notice--tip',
        'caution'  => 'notice notice--caution'
      }

      ICON_MAP = {
        'info'     => 'notice-note-icon',
        'warning'  => 'notice-warning-icon',
        'danger'   => 'notice-caution-icon',
        'note'     => 'notice-note-icon',
        'tip'      => 'notice-tip-icon',
        'caution'  => 'notice-caution-icon'
      }

      TITLE_MAP = {
        'info'     => 'Note',
        'warning'  => 'Warning',
        'danger'   => 'Caution',
        'note'     => 'Note',
        'tip'      => 'Tip',
        'caution'  => 'Caution'
      }

      def initialize(tag_name, markup, tokens)
        super
        @config = {}
        override_config(@@DEFAULTS)

        params = markup.scan(/([a-z]+)="(.+?)"/)
        if params.size > 0
          config_hash = {}
          params.each do |(key,val)|
            sym_key = key.to_sym
            if sym_key == :active
              config_hash[sym_key] = (val.downcase == 'true')
            else
              config_hash[sym_key] = val
            end
          end
          override_config(config_hash)
        end
      end

      def render(context)
        raw_content = super

        rendered_content = Jekyll::Converters::Markdown::KramdownParser
                            .new(Jekyll.configuration)
                            .convert(raw_content)

        level_str  = (@config[:level] || 'info').downcase
        container_class = CLASS_MAP[level_str] || 'notice notice--info'
        icon_id    = ICON_MAP[level_str]       || 'notice-info-icon'
        title_text = TITLE_MAP[level_str]      || 'Info'

        <<~HTML
          <div class="#{container_class}" markdown="0">
            <div class="notice__content">
              <div class="notice__title">
                <svg class="notice__icon">
                  <use xlink:href="/images/sprite.svg##{icon_id}"></use>
                </svg>
                <div>#{title_text}</div>
              </div>
              <div>
                #{rendered_content}
              </div>
            </div>
          </div>
        HTML
      end

      private

      def override_config(config)
        config.each { |key,value| @config[key] = value }
      end
    end
  end
end

Liquid::Template.register_tag('alert', Jekyll::Alert::AlertTag)
