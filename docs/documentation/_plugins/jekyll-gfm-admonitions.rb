 # frozen_string_literal: true

require 'octicons'
require 'cssminify2'
require 'liquid/template'

ADMONITION_ICONS = {
  'important' => 'report',
  'note' => 'info',
  'tip' => 'light-bulb',
  'warning' => 'alert',
  'caution' => 'stop'
}.freeze

# JekyllGFMAdmonitions is a module that provides functionality to process and
# convert GitHub-flavored markdown admonitions into HTML within Jekyll.
module JekyllGFMAdmonitions
  # GFMAdmonitionConverter is a Jekyll generator that converts custom
  # admonition blocks in markdown (e.g., `> [!IMPORTANT]`) into styled HTML
  # alert boxes with icons.
  #
  # This generator processes both posts and pages, replacing admonition
  # syntax with HTML markup that includes appropriate iconography and CSS styling.
  class GFMAdmonitionConverter < Jekyll::Generator
    safe true
    priority :lowest
    @admonition_pages = []

    class << self
      attr_reader :admonition_pages
    end


    def generate(site)
      init_converter(site)
      process_posts(site)
      process_pages(site)
      Jekyll.logger.info 'GFMA:', 'Converted admonitions in' \
        " #{self.class.admonition_pages.length} file(s)."
    end

    def init_converter(site)
      @markdown = site.converters.find { |c| c.is_a?(Jekyll::Converters::Markdown) }
      return if @markdown

      raise 'Markdown converter not found. Please ensure that you have a markdown' \
              ' converter configured in your Jekyll site.'
    end

    def process_posts(site)
      site.posts.docs.each do |doc|
        Jekyll.logger.debug 'GFMA:', "Processing post '#{doc.path}' (#{doc.content.length} characters)."
        process_doc(doc)
      end
    end

    def process_pages(site)
      site.pages.each do |page|
        Jekyll.logger.debug 'GFMA:', "Processing page '#{page.path}' (#{page.content.length} characters)."
        process_doc_content(page)
      end
    end

    def process_doc_content(doc)
      original_content = doc.content.dup
      process_doc(doc)

      return unless doc.content != original_content

      # Store a reference to all the pages we modified, to inject the CSS post render
      # (otherwise GitHub Pages sanitizes the CSS into plaintext)
      self.class.admonition_pages << doc
    end

    def process_doc(doc)
      code_blocks = []
      # Temporarily replace code blocks by a tag, so that we don't process any admonitions
      # inside of code blocks.
      doc.content.gsub!(/(?:^|\n)(?<!>)\s*```.*?```/m) do |match|
        code_blocks << match
        "```{{CODE_BLOCK_#{code_blocks.length - 1}}}```"
      end

      convert_admonitions(doc)

      # Put the code blocks back in place
      doc.content.gsub!(/```\{\{CODE_BLOCK_(\d+)}}```/) do
        code_blocks[::Regexp.last_match(1).to_i]
      end
    end

    def convert_admonitions(doc)
      doc.content.gsub!(/>\s*\[!(IMPORTANT|NOTE|WARNING|TIP|CAUTION)\]\s*\n((?:>.*\n?)*)/) do
        type = ::Regexp.last_match(1).downcase
        title = type.capitalize
        text = ::Regexp.last_match(2).gsub(/^>\s*/, '').strip
        icon = Octicons::Octicon.new(ADMONITION_ICONS[type]).to_svg
        Jekyll.logger.debug 'GFMA:', "Converting #{type} admonition."

        admonition_html(type, title, text, icon)
      end
    end

    def admonition_html(type, title, text, icon)
      "<div markdown='0' class='markdown-alert markdown-alert-#{type}'><p class='markdown-alert-title'>#{icon} #{title}</p><p>#{@markdown.convert(text)}</p></div>\n\n"
    end
  end

  # Insert the minified CSS before the closing head tag of all pages we put admonitions on
  Jekyll::Hooks.register :site, :post_render do
    Jekyll.logger.info 'GFMA:', "Inserting admonition CSS in #{GFMAdmonitionConverter.admonition_pages.length} page(s)."

    GFMAdmonitionConverter.admonition_pages.each do |page|
      Jekyll.logger.debug 'GFMA:', "Appending admonition style to '#{page.path}'."
      css = File.read(File.expand_path('../_assets/css/admonitions.css', __dir__))

      page.output.gsub!(%r{<head>(.*?)</head>}m) do |match|
        "#{match[0..-7]}<style>#{CSSminify2.compress(css)}</style>#{match[-7..]}"
      end

      # If no <head> tag is found, insert the CSS at the start of the output
      if !page.output.match(%r{<head>(.*?)</head>}m)
        Jekyll.logger.debug 'GFMA:', "No <head> tag found in '#{page.path}', inserting CSS at the beginning of the page."
        page.output = "<head><style>#{CSSminify2.compress(css)}</style></head>" + page.output
      end
    end
  end
end
