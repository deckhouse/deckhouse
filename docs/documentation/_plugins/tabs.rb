require_relative "utils"

module Jekyll
  module Tabs
    class TabsTag < Liquid::Block
      include JekyllLiquidBlockUtils

      def initialize(tag_name, markup, tokens)
        super

        markup = markup.strip
        raise SyntaxError, "#{tag_name}: group name is required. Usage: {% tabs group_name %}" if markup.empty?

        m = markup.match(/^(\S+)(?:\s+store_key="([^"]*)")?/)
        raise SyntaxError, "#{tag_name}: invalid syntax. Usage: {% tabs group_name [store_key=\"key\"] %}" unless m

        @group = m[1]
        @store_key = m[2]
      end

      def render(context)
        context.registers[:tabs_stack] ||= []
        context.registers[:tabs_stack].push([])

        super

        tabs = context.registers[:tabs_stack].pop
        return "" if tabs.empty?

        btn_class   = "tabs__btn_#{@group}"
        cont_class  = "tabs__content_#{@group}"

        buttons = tabs.each_with_index.map do |tab, i|
          block_id   = "block_#{@group}_#{slugify(tab[:label])}"
          active     = i == 0 ? " active" : ""
          store_args = @store_key && tab[:store_val] ? ", '#{@store_key}', '#{tab[:store_val]}'" : ""
          %Q(<a href="javascript:void(0)" class="tabs__btn #{btn_class}#{active}" onclick="openTabAndSaveStatus(event, '#{btn_class}', '#{cont_class}', '#{block_id}'#{store_args});">#{tab[:label]}</a>)
        end

        panels = tabs.each_with_index.map do |tab, i|
          block_id = "block_#{@group}_#{slugify(tab[:label])}"
          active   = i == 0 ? " active" : ""
          %Q(<div id="#{block_id}" class="tabs__content #{cont_class}#{active}" markdown="0">#{tab[:html]}</div>)
        end

        %Q(<div markdown="0"><div class="tabs">#{buttons.join}</div>#{panels.join}</div>)
      end

      private

      def slugify(text)
        text.downcase.strip.gsub(/[^a-z0-9\s_-]/, '').gsub(/[\s-]+/, '_')
      end
    end

    class TabTag < Liquid::Block
      include JekyllLiquidBlockUtils

      def initialize(tag_name, markup, tokens)
        super

        m = markup.strip.match(/^"([^"]+)"(?:\s+store_val="([^"]*)")?/)
        raise SyntaxError, "#{tag_name}: tab label is required. Usage: {% tab \"Label\" [store_val=\"val\"] %}" unless m

        @label     = m[1]
        @store_val = m[2]
      end

      def render(context)
        raise SyntaxError, "{% tab %} must be used inside {% tabs %}" unless context.registers[:tabs_stack]&.last

        content = dedent(super)

        site = context.registers[:site]
        converter = site.find_converter_instance(::Jekyll::Converters::Markdown)
        html = converter.convert(content).gsub(/\n/, '')

        context.registers[:tabs_stack].last << { label: @label, html: html, store_val: @store_val }

        ""
      end
    end
  end
end

Liquid::Template.register_tag('tabs', Jekyll::Tabs::TabsTag)
Liquid::Template.register_tag('tab', Jekyll::Tabs::TabTag)
