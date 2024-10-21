
module Jekyll
  module CustomFilters
    STRIP_HTML_BLOCKS       = Regexp.union(
      %r{<script.*?</script>}m,
      /<!--.*?-->/m,
      %r{<style.*?</style>}m
    )
    STRIP_MD_TABLES       = Regexp.union(
      %r{\|\ ?[:+-= ]+\ ?\|},
      %r#[:+-= ]{4,}#,
      %r{\|\|+}
    )
    STRIP_LIQUID_TAGS       = Regexp.union(
      /\{{.*?}}/m,
      /\{%.*?%}/m
    )
    STRIP_HTML_TAGS = /<.*?>/m

    def true_relative_url(path)
        if !path.instance_of? String
            return "unexpected argument #{path}"
            raise "true_relative_url filter failed: unexpected argument #{path}"
        end

        # remove first slash if exist
        page_path_relative = @context.registers[:page]["url"].gsub(%r!^/!, "")
        page_depth = page_path_relative.scan(%r!/!).count - 1
        prefix = ""
        page_depth.times{ prefix = prefix + "../" }
        prefix + path.sub(%r!^/!, "./")
    end

    def endswith(text, query)
      return text.end_with? query
    end

    def camel_to_snake_case(text)
      return text.to_s.gsub(/([A-Z]+)([A-Z][a-z])/,'\1_\2').
                         gsub(/([a-z\d])([A-Z])/,'\1_\2').
                         tr("-", "_").downcase
    end

    def normalizeSearchContent(text)
      return text.to_s.gsub(STRIP_HTML_BLOCKS, ' ').
                       gsub(STRIP_HTML_TAGS, ' ').
                       gsub(STRIP_MD_TABLES,' ').
                       gsub(STRIP_LIQUID_TAGS, ' ').
                       gsub(/\n/,' ').
                       gsub(/\s\s+/,' ').strip
    end

    def startswith(text, query)
      return text.start_with? query if text
    end

    # get_lang_field_or_raise_error filter returns a field from argument hash
    # returns nil if hash is empty
    # returns hash[page.lang] if hash has the field
    # returns hash["all"] if hash has the field
    # otherwise, raise an error
    def get_lang_field_or_raise_error(hash)
        if !(hash == nil or hash.instance_of? Hash)
            raise "get_lang_field_or_raise_error filter failed: unexpected argument '#{hash}'"
        end

        if hash == nil or hash.length == 0
            return
        end

        lang = @context.registers[:page]["lang"]
        if hash.has_key?(lang)
            return hash[lang]
        elsif hash.has_key?("all")
            return hash["all"]
        else
            raise "get_lang_field_or_raise_error filter failed: the argument '#{hash}' does not have '#{lang}' or 'all' field"
        end
    end
  end
end

Liquid::Template.register_filter(Jekyll::CustomFilters)
