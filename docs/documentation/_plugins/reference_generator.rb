module ReferenceGenerator

  class ReferenceGenerator < Jekyll::Generator
    safe true

    def generate(site)
      # Generate pages for D8
      languages = ['ru', 'en']
      converter = site.find_converter_instance(::Jekyll::Converters::Markdown)

      puts "Generating reference for D8..."
      languages.each do |lang|
        site.pages << ReferenceD8Page.new(site, lang, converter)
      end
    end

  end

  class ReferenceD8Page < Jekyll::Page
    def initialize(site, lang, converter)
      @site = site
      @converter = converter
      @baseUrl = '/reference/cli-tools/d8'
      @referenceData = site.data['reference']['d8']
      @base = site.source
      @lang = lang
      @sidebar = 'kubernetes-platform'
      @fileName =  "index.html"

      @path = "#{@baseUrl}/#{@fileName}"
      self.process(@path)

      self.data = {
        'title' => "Reference Deckhouse CLI",
        'searchable' => true,
        'permalink' => "%s/%s" % [ @lang, @path ],
        'url' => "%s/%s" % [ @lang, @path ],
        'layout' => 'page',
        'lang' => @lang,
        'name' => @fileName,
        'sidebar' => @sidebar,
        'search_bage_enabled' => true,
        'sitemap_include' => false
      }

      self.content = renderD8Section(@referenceData, 1)

      Jekyll::Hooks.trigger :pages, :post_init, self
    end

    def renderD8Section(data, depth)
      headerLevel = depth + 1
      result = ""
      result += "\n" + data['description'] + "\n\n" if data['description']

      # Render flags
      if data['flags'] && data['flags'].size > 0

        result += '<p>'

        if depth == 1
          result += %Q(<strong>Common options:</strong></br>\n)
        else
          result += %Q(<strong>Options</strong></br>\n)
        end

        result += '<ul>'
        data['flags'].each do |flagName, flagData|
          # Add header
          result += %Q(<li><p><code>--#{flagName}</code>)
          result += %Q(, <code>-#{flagData['shorthand']}</code>) if flagData.has_key?('shorthand') && flagData['shorthand'].size > 0
          result += %Q( — #{@converter.convert(flagData['description']).sub(/^<p>/,'').sub(/<\/p>$/,'')}</p></li>\n)
        end
        result += '</ul>'
      end
      result += '</p>'

      # Render commands
      if data['subcommands'] && data['subcommands'].size > 0
        data['subcommands'].each do | subcommand |
          #result += %Q(#{'#' * headerLevel} #{subcommand['name']}#{if subcommand['aliases'] then ' (' + subcommand['aliases'].join(',') + ')' end}\n\n)
          result += %Q(<h#{headerLevel}>#{subcommand['name']}</h#{headerLevel}>\n)
          #  result += %Q(#{'#' * headerLevel} #{subcommand}\n\n)
          result += renderD8Section(subcommand, depth+1)
        end
      end
      result
    end

  end
end
