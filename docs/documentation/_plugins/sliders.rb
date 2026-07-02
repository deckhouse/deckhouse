require "cgi"
require_relative "utils"

module Jekyll
  module Sliders
    class SlidersTag < Liquid::Block
      include JekyllLiquidBlockUtils

      def initialize(tag_name, markup, tokens)
        super

        @markup = markup
      end

      def render(context)
        context.registers[:sliders_stack] ||= []
        context.registers[:sliders_stack].push([])

        super

        slides = context.registers[:sliders_stack].pop
        return "" if slides.empty?

        interpolated = Liquid::Template.parse(@markup).render(context)
        m = interpolated.strip.match(/^"([^"]*)"/)
        title = m ? m[1] : nil

        page = context.registers[:page]
        lang = page && page["lang"] == "ru" ? "ru" : "en"

        aria_slides = lang == "ru" ? "Слайды" : "Slides"
        aria_prev   = lang == "ru" ? "Предыдущий слайд" : "Previous slide"
        aria_next   = lang == "ru" ? "Следующий слайд" : "Next slide"

        title_html = ""
        if title && !title.empty?
          title_html = %Q(<div class="slider__title">#{CGI.escapeHTML(title)}</div>)
        end

        slides_html = slides.map do |slide|
          img_attrs = %Q(src="#{slide[:img]}" alt="#{CGI.escapeHTML(slide[:alt] || '')}" decoding="async")
          caption   = slide[:html].to_s.strip.empty? ? "" : %Q(<figcaption class="slider__caption">#{slide[:html]}</figcaption>)
          %Q(<div class="slider__slide"><figure class="slider__figure"><img #{img_attrs} />#{caption}</figure></div>)
        end.join

        nav_html = ""
        if slides.size > 1
          nav_html = %Q(<div class="slider__nav" role="group" aria-label="#{aria_slides}"><button type="button" class="slider__prev" aria-label="#{aria_prev}">←</button><div class="slider__pagination" aria-live="polite"></div><button type="button" class="slider__next" aria-label="#{aria_next}">→</button></div>)
        end

        html = %Q(<div markdown="0" class="slider" data-slider>#{title_html}<div class="slider__viewport"><div class="slider__track">#{slides_html}</div></div>#{nav_html}</div>)
        collapse_inter_block_newlines(html)
      end
    end

    class SliderTag < Liquid::Block
      include JekyllLiquidBlockUtils

      def initialize(tag_name, markup, tokens)
        super

        @tag_name = tag_name
        @markup = markup
      end

      def render(context)
        raise SyntaxError, "{% slider %} must be used inside {% sliders %}" unless context.registers[:sliders_stack]&.last

        interpolated = Liquid::Template.parse(@markup).render(context)

        params = {}
        interpolated.scan(/([a-z]+)\s*=\s*"([^"]*)"/).each { |k, v| params[k.to_sym] = v }

        raise SyntaxError, "#{@tag_name}: img attribute is required. Usage: {% slider img=\"path\" %}" unless params[:img] && !params[:img].empty?

        content = dedent(super)

        site = context.registers[:site]
        converter = site.find_converter_instance(::Jekyll::Converters::Markdown)
        html = collapse_inter_block_newlines(converter.convert(content))

        context.registers[:sliders_stack].last << { img: params[:img], alt: params[:alt], html: html }

        ""
      end
    end
  end
end

Liquid::Template.register_tag('sliders', Jekyll::Sliders::SlidersTag)
Liquid::Template.register_tag('slider', Jekyll::Sliders::SliderTag)
