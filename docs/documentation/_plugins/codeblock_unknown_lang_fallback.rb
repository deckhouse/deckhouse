# frozen_string_literal: true

# Normalizes unknown fenced-code languages to plaintext highlighting
# with the same highlighter-rouge wrapper shape as known languages.

require "cgi"
require "rouge"

module Deckhouse
  module UnknownCodeblockFallback
    CODE_BLOCK_REGEX = %r{<pre(?<pre_attrs>[^>]*)>\s*<code(?<code_attrs>[^>]*)>(?<body>.*?)</code>\s*</pre>}m

    module_function

    def fallback_unknown_codeblocks!(html)
      html.gsub(CODE_BLOCK_REGEX) do |original|
        pre_attrs = Regexp.last_match(:pre_attrs)
        code_attrs = Regexp.last_match(:code_attrs)
        body = Regexp.last_match(:body)
        lang = extract_language(code_attrs)

        next original if lang.nil? || lang.empty?

        raw_code = CGI.unescapeHTML(body)

        # Keep known lexers untouched.
        next original if Rouge::Lexer.find_fancy(lang, raw_code)

        highlighted = Rouge::Formatters::HTML.new.format(Rouge::Lexers::PlainText.lex(raw_code))
        safe_lang = CGI.escapeHTML(lang)
        normalized_pre_attrs = ensure_class(pre_attrs, "highlight")
        normalized_code_attrs = replace_language_class(code_attrs, lang)
        normalized_code_attrs = ensure_data_lang(normalized_code_attrs, safe_lang)

        %(<div class="language-#{safe_lang} highlighter-rouge"><div class="highlight"><pre#{normalized_pre_attrs}><code#{normalized_code_attrs}>#{highlighted}</code></pre></div></div>)
      end
    end

    def extract_language(code_attrs)
      class_attr = code_attrs[/\bclass=(["'])(.*?)\1/m, 2]
      return nil if class_attr.nil? || class_attr.empty?

      lang_class = class_attr.split(/\s+/).find { |token| token.start_with?("language-") }
      return nil if lang_class.nil?

      lang_class.delete_prefix("language-")
    end

    def ensure_class(attrs, class_name)
      class_attr = attrs[/\bclass=(["'])(.*?)\1/m, 2]
      return %(#{attrs} class="#{class_name}") if class_attr.nil?

      classes = class_attr.split(/\s+/)
      return attrs if classes.include?(class_name)

      updated = (classes + [class_name]).join(" ")
      attrs.sub(/\bclass=(["'])(.*?)\1/m, %(class="#{updated}"))
    end

    def replace_language_class(code_attrs, original_lang)
      class_attr = code_attrs[/\bclass=(["'])(.*?)\1/m, 2]
      if class_attr.nil?
        return %(#{code_attrs} class="language-plaintext")
      end

      classes = class_attr.split(/\s+/).reject { |token| token == "language-#{original_lang}" }
      classes.reject! { |token| token.start_with?("language-") }
      classes << "language-plaintext"
      code_attrs.sub(/\bclass=(["'])(.*?)\1/m, %(class="#{classes.join(" ")}"))
    end

    def ensure_data_lang(code_attrs, lang)
      return code_attrs if code_attrs.match?(/\bdata-lang=(["']).*?\1/m)

      %(#{code_attrs} data-lang="#{lang}")
    end
  end
end

Jekyll::Hooks.register [:pages, :documents], :post_render do |doc|
  next unless doc.output_ext == ".html"
  next unless doc.output

  doc.output = Deckhouse::UnknownCodeblockFallback.fallback_unknown_codeblocks!(doc.output)
end
