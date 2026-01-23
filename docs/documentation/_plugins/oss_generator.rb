# Copyright 2024 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

module OSSGenerator
  class OSSGenerator < Jekyll::Generator
    safe true
    priority :low

    def generate(site)
      # Check if OSS data exists
      return unless site.data['modules']['metadata']['modules']

      languages = ['en', 'ru']

      # Get all modules that have OSS data
      #site.data['oss'].each do |module_name, oss_items|
      site.data['modules']['metadata']['modules'].each do |module_name, module_data|
        oss_items = module_data['oss']
        next unless oss_items && oss_items.size > 0

        languages.each do |lang|
          # Generate OSS page for this module and language
          site.pages << OSSPage.new(site, module_name, oss_items, lang)
        end
      end
    end
  end

  class OSSPage < Jekyll::Page
    def initialize(site, module_name, oss_items, lang)
      @site = site
      @base = site.source
      @lang = lang
      @module_name = module_name
      @oss_items = oss_items

      # Determine the path based on language
      # Pages should be in: en/modules/<module-name>/OSS.html or ru/modules/<module-name>/OSS.html
      @dir = "#{@lang}/modules/#{@module_name}/"
      @name = "OSS.md"

      self.process(@name)

      # Set page data
      self.data = {
        'title' => get_title(lang),
        'layout' => 'page',
        'lang' => lang,
        'sidebar' => 'embedded-modules',
        'module-kebab-name' => @module_name,
        'module-snake-name' => @module_name.gsub(/-[a-z]/) { |m| m.upcase }.gsub(/-/, ''),
        'permalink' => "#{@dir}oss.html",
        'searchable' => false,
        'sitemap_include' => true
      }

      # Set content using the module-oss.liquid include
      # Use markdown format so Liquid includes are processed
      self.content = <<~CONTENT
        {% include module-oss.liquid %}
      CONTENT

      Jekyll::Hooks.trigger :pages, :post_init, self
    end

    private

    def get_title(lang)
      if lang == 'ru'
        "Используемые компоненты"
      else
        "Open Source Components"
      end
    end
  end
end
