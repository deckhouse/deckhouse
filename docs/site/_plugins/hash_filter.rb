module Jekyll
  module HashFilter
    require 'digest'

    def sha256(input)
      input_str = input.is_a?(String) ? input : input.to_s
      Digest::SHA256.hexdigest(input_str)
    end
  end
end

Liquid::Template.register_filter(Jekyll::HashFilter)
