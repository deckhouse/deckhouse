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

      self.content = renderD8Section(@referenceData, 1, [], {})
      Jekyll::Hooks.trigger :pages, :post_init, self
    end

    def extract_first_word(name)
      name.split(' ').first
    end
    # Build a header
    def build_header_title(parent_titles, current_name)
      parts = parent_titles.map { |n| extract_first_word(n) }
      parts << extract_first_word(current_name)
      "d8 #{parts.join(' ')}"
    end

    def prepare_signature(signature)
      # Shield HTML characters with CGI.escapeHTML
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

    def collect_global_flags(data, depth, inherited_flags)
      current_flags = {}
      
      # Add flags from current level
      if data['flags']
        data['flags'].each do |flag_name, flag_data|
          current_flags[flag_name] = flag_data
          if flag_data['global'] == true
            inherited_flags[flag_name] = flag_data
          end
        end
      end
      
      # Join flags from different levels
      all_flags = inherited_flags.merge(current_flags)
      
      # Get global flags from subcommands
      if data['subcommands']
        data['subcommands'].each do |subcommand|
          collect_global_flags(subcommand, depth + 1, inherited_flags.dup)
        end
      end
      
      all_flags
    end

    def renderD8Section(data, depth, parent_titles, inherited_flags)
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

      # Get all flags (current + global)
      current_flags = {}
      if data['flags']
        data['flags'].each do |flag_name, flag_data|
          current_flags[flag_name] = flag_data
          if flag_data['global'] == true
            inherited_flags[flag_name] = flag_data
          end
        end
      end
      all_flags = inherited_flags.merge(current_flags)

      # Render flags
      if all_flags.size > 0
        result += '<p>'
        result += depth == 1 ? '<strong>Common options:</strong></br>' : '<strong>Options</strong></br>'
        result += '<ul>'
        all_flags.each do |flag_name, flag_data|
          result += %Q(<li><p><code>--#{flag_name}</code>)
          result += %Q(, <code>-#{flag_data['shorthand']}</code>) if flag_data['shorthand'].to_s.size > 0
          result += %Q( — #{@converter.convert(flag_data['description']).sub(/^<p>|<\/p>$/, '')})
          result += %Q( <span class="global-flag"><i>(global option)</i></span>) if flag_data['global'] == true
          result += %Q(</p></li>)
        end
        result += '</ul></p>'
      end
      # Render commands
      if data['subcommands'] && data['subcommands'].size > 0
        data['subcommands'].each do |subcommand|
          new_parent_titles = depth == 1 ? [] : parent_titles + [data['name']]
          result += renderD8Section(subcommand, depth + 1, new_parent_titles, inherited_flags.dup)
        end
      end

      result
    end
  end
end