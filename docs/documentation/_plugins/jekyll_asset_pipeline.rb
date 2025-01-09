require 'jekyll_asset_pipeline'

module JekyllAssetPipeline
  class SassConverter < JekyllAssetPipeline::Converter
    require 'sass'

    def self.filetype
      '.scss'
    end

    def convert
      return Sass::Engine.new(@content, syntax: :scss, load_paths: [@dirname]).render
    end
  end

  class CssCompressor < JekyllAssetPipeline::Compressor
    require 'cssminify2'

    def self.filetype
      '.css'
    end

    def compress
      return CSSminify2.compress(@content)
    end
  end

  class CssTagTemplate < JekyllAssetPipeline::Template
    def self.filetype
      '.css'
    end

    def html
      "#{output_path}/#{@filename}"
    end
  end

  class JsTagTemplate < JekyllAssetPipeline::Template
    def self.filetype
      '.js'
    end

    def html
      "#{output_path}/#{@filename}"
    end
  end
end
