module Jekyll
  module HashFilter
    require 'digest'

    def sha256(input)
      Digest::SHA256.hexdigest(input)
    end
  end
end

Liquid::Template.register_filter(Jekyll::HashFilter)
