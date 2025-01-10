module Comparator
  class Generator < Jekyll::Generator
    safe true

    def generate(site)
        comparingPages = []
        converter = Jekyll::Converters::Markdown::KramdownParser.new(Jekyll.configuration())
        site.pages.each do |page|

            next if ! page.path.end_with?('.md')

            if page.path.end_with?('_RU.md') then
              pageToComparePath = page.path.sub(%r{_RU.md$}, '.md')
            elsif page.path.end_with?('.md')
              pageToComparePath = page.path.sub(%r{.md$}, '_RU.md')
            end

            pageToCompare = site.pages.find { |item| item.path == pageToComparePath }

            if ! pageToCompare then
              if page.path.include?('modules_ru') then
                pageToComparePath = page.path.sub('modules_ru', 'modules_en')
              elsif page.path.include?('modules_en')
                pageToComparePath = page.path.sub('modules_en', 'modules_ru')
              end
            end

            pageToCompare = site.pages.find { |item| item.path == pageToComparePath }

            if ! pageToCompare then
                puts "Skip comparing for #{page.path}"
                next
            end

            comparingPage = ComparatorPage.new(site, page, pageToCompare, converter)
            next if comparingPages.find { |item| item.path == comparingPage.path if item.path }
            comparingPages << comparingPage
        end

          site.pages += comparingPages
    end
    def inspect_object(input, depth)
      if depth < 1 then return end
      input.instance_variables.inject([]) do |result, item|
        variable_content = input.instance_variable_get(item).instance_variables.length > 0 ?
        self.inspect_object(input.instance_variable_get(item), depth-1)
                           : input.instance_variable_get(item)
        result << "\t\t#{item} = #{variable_content}"
        result
      end.join("\n")
    end

  end

  class ComparatorPage < Jekyll::Page
    @@SPLIT_PATTERN = %r{\n{2,}#?|\n#}
    def initialize(site, page, pageToCompare, converter)

      @site = site
      @base = site.source
      @converter = converter

      if page.permalink then
        if page.permalink.end_with?("/") then
          @dir = page.permalink
        else
          @dir = File.dirname(page.permalink)
        end
      else
        if page.path.end_with?("/") then
          @dir = page.path.split(/pages/)[-1]
        else
          @dir = File.dirname(page.path.split(/pages/)[-1]).gsub(%r{/[^\/]+$},'/')
        end
      end

      @dir = @dir.sub(%r{^\.$},'').sub(%r{^/?(ru|en)/},'').sub(%r{^(ru|en)$},'').sub(%r{^/},'') if @dir
      @dir  = "compare/#{@dir}"
      @name = page.name.downcase.sub(%r{\_(ru|en)\.([a-zA-Z]+)$},'.\2').sub(%r{.md$},'.html')

      self.process(@name)
      @path = site.in_source_dir(@base, @dir, @name)
      @path = File.join(@path, "index.html") if url.end_with?("/")

      self.data = { 'title' => "Compare languages | %s" %  page.data['title'],
                    'layout' => 'compare',
                    'searchable' =>  false,
                    'sitemap_include' => false,
                    'sidebar' => 'none',
                    'output' => 'web',
                    'multilang' => false}

      self.content = make_content(page, pageToCompare)


      Jekyll::Hooks.trigger :pages, :post_init, self

    end

    def make_content(page1, page2)
      para1 = (page1 and page1.content) ? page1.content.split(@@SPLIT_PATTERN) : []
      para2 = (page2 and page2.content) ? page2.content.split(@@SPLIT_PATTERN) : []
      maxlength = para1.length > para2.length ? para1.length : para2.length

      result = %q(
      <link rel="stylesheet" type="text/css" href="/assets/css/dev.css">{% raw %}
      <table class="lang__compare">
)

      (0..maxlength-1).each do |index|
        next if para1[index] and para2[index] and para1[index].length==0 and para2[index].length==0
        result << "<tr" + (para1[index] == para2[index] ? ' class="equal"' : '' ) + '><td>'
        result << (para1[index] ? @converter.convert(sanitize(para1[index])) : '&nbsp;')
        result << "</td><td>"
        result << (para2[index] ? @converter.convert(sanitize(para2[index])) : '&nbsp;')
        result << "</td></tr>"
      end
      result << "</table>{% endraw %}"
      result
    end

    def sanitize(input)
      input.gsub(%r({%[^%]+%}),'').gsub(%r(\r+),"\r").gsub(%r(\n+),"\n").gsub(%r{^\s+(#+\ )?},'').gsub(%r{```},'').lstrip
    end
  end

end

