module Jekyll
  class CompactString < Liquid::Block

    def render(context)
       super.gsub /\n/, " "
       super.gsub /\s{2,}/, " "
    end

  end
end

Liquid::Template.register_tag('compact_string', Jekyll::CompactString)