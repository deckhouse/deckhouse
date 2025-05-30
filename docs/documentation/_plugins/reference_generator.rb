require 'cgi'

module ReferenceGenerator
  class ReferenceGenerator < Jekyll::Generator
    safe true

    def generate(site)
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
      @fileName = "index.html"

      @path = "#{@baseUrl}/#{@fileName}"
      self.process(@path)

      self.data = {
        'title' => "Reference Deckhouse CLI",
        'searchable' => true,
        'permalink' => "%s/%s" % [@lang, @path],
        'url' => "%s/%s" % [@lang, @path],
        'layout' => 'page',
        'lang' => @lang,
        'name' => @fileName,
        'sidebar' => @sidebar,
        'search_bage_enabled' => true,
        'sitemap_include' => false
      }

      # Add alert for ru page
      @language_alert = if @lang == 'ru'
        "\n{% alert level=\"info\" %}\nСтраница генерируется автоматически, информация представлена только на английском языке.\n{% endalert %}\n"
      else
        ""
      end

      self.content = @language_alert + renderD8Section(@referenceData, 1, [])
      Jekyll::Hooks.trigger :pages, :post_init, self
    end

    def extract_first_word(name)
      name.split(' ').first
    end

    def build_header_title(parent_titles, current_name)
      parts = parent_titles.map { |n| extract_first_word(n) }
      parts << extract_first_word(current_name)
      "d8 #{parts.join(' ')}"
    end

    def prepare_signature(signature)
      escaped = CGI.escapeHTML(signature)
      escaped += ' [options]' unless escaped.include?('[options]')
      escaped
    end
    # Build a command's signature
    def build_full_signature(parent_titles, original_name)
      full_path = (parent_titles + [original_name]).join(' ')
      signature = "d8 #{full_path}" unless parent_titles.empty?
      prepare_signature(signature) if signature
    end

    def render_flags(flags, depth)
      return '' unless flags && flags.size > 0

      # Separate flags to local and global
      regular_flags = flags.reject { |_, f| f['global'] == true }
      global_flags = flags.select { |_, f| f['global'] == true }

      result = ''

      # Render flags if exist
      if regular_flags.any?
        result += '<p>'
        result += depth == 1 ? '<strong>Common options:</strong></br>' : '<strong>Options</strong></br>'
        result += '<ul>'
        regular_flags.each do |flag_name, flag_data|
          result += render_flag_item(flag_name, flag_data)
        end
        result += '</ul></p>'
      end

      # Render global flags if exist
      if global_flags.any?
        result += '<p><strong>Global options</strong></br><ul>'
        global_flags.each do |flag_name, flag_data|
          result += render_flag_item(flag_name, flag_data)
        end
        result += '</ul></p>'
      end

      result
    end

    def render_flag_item(flag_name, flag_data)
      result = %Q(<li><p><code>--#{flag_name}</code>)
      result += %Q(, <code>-#{flag_data['shorthand']}</code>) if flag_data['shorthand'].to_s.size > 0
      result += %Q( — #{@converter.convert(flag_data['description']).sub(/^<p>|<\/p>$/, '')}</p></li>)
    end

    def renderD8Section(data, depth, parent_titles)
      result = ""

      unless depth == 1
        # Apply styles to headings depending on nesting 
        header_tag = depth == 2 ? 'h2' : 'h3'
        style = depth == 2 ? ' style="text-decoration: underline;"' : ''
        
        header_title = build_header_title(parent_titles, data['name'])
        
        result += %Q(<#{header_tag}#{style}>#{header_title}</#{header_tag}>\n)
        
        if header_tag == 'h3'
          signature = build_full_signature(parent_titles, data['name'])
          result += %Q(<b>Usage:</b><div class="language-shell highlighter-rouge"><div class="highlight"><pre class="highlight"><code>#{signature}</code></pre></div></div>\n\n)
        end
      end

      result += "\n#{data['description']}\n\n" if data['description']

      # Render flags
      result += render_flags(data['flags'], depth) if data['flags'] && data['flags'].size > 0

      if data['subcommands'] && data['subcommands'].size > 0
        data['subcommands'].each do |subcommand|
          new_parent_titles = depth == 1 ? [] : parent_titles + [data['name']]
          result += renderD8Section(subcommand, depth + 1, new_parent_titles)
        end
      end

      result
    end
  end
end