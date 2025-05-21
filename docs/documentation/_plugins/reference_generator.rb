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

      self.content = renderD8Section(@referenceData, 1, [])

      Jekyll::Hooks.trigger :pages, :post_init, self
    end

    def clean_title(title)
      # Delete everything in brackets (including nested brackets)
      while title.gsub!(/[\[\(][^\[\]\(\)]*([\[\(][^\[\]\(\)]*[\]\)][^\[\]\(\)]*)*[\]\)]/, ''); end
      
      # Delete the word SUBCOMMAND (if the header does not consist only of it)
      if title != "SUBCOMMAND"
        title.gsub!(/\bSUBCOMMAND\b/, '')
      end
      
      # Delete enums with | (but keep the text after them)
      title.gsub!(/(^|\s)([^\s|]+\|)+[^\s|]+(\s|$)/) do |match|
        # Save spaces and text after the enums
        match.start_with?(' ') ? ' ' : ''
      end
      
      # Remove the extra spaces
      title.gsub(/\s+/, ' ').strip
    end

    def renderD8Section(data, depth, parent_titles)
      result = ""

      # Skip rendering the top-level (d8) header
      unless depth == 1
        # Build the full title from parent titles and current name
        full_title = (parent_titles + [data['name']]).join(' ')

        # Determine header level and style
        header_tag = depth == 2 ? 'h2' : 'h3'
        style = depth == 2 ? ' style="text-decoration: underline;"' : ''
        
        # Add 'd8' prefix for h3 headers
        full_title = "d8 #{full_title}" if header_tag == 'h3'
        
        # Clean the title
        cleaned_title = clean_title(full_title)
        
        result += %Q(<#{header_tag}#{style}>#{cleaned_title}</#{header_tag}>\n)
      end

      # Add description after header
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
        data['subcommands'].each do |subcommand|
          # Pass current parent titles plus current name (unless it's top level)
          new_parent_titles = depth == 1 ? [] : parent_titles + [data['name']]
          result += renderD8Section(subcommand, depth + 1, new_parent_titles)
        end
      end
      result
    end
  end
end