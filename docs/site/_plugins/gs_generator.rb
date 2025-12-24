module GSGenerator
  class DKP_GSGenerator < Jekyll::Generator
    safe true

    def generate(site)
      site.data['getting_started']['dkp_data']['installTypes'].each do |installTypeKey, installTypeData|

        next unless installTypeData['steps']

        puts "[GS DKP] Generating pages for %s..." % [installTypeKey]

        installTypeData['steps'].each do |stepName, stepData|
          languages = installTypeData['languages'] ? installTypeData['languages'] : ['ru', 'en']
          languages.each do |lang|
            site.pages << DKP_GSPage.new(site, site.data['getting_started']['dkp_data']['global'], installTypeKey, installTypeData, stepName, lang)
          end
        end
      end
    end
  end

  class DVP_GSGenerator < Jekyll::Generator
    safe true

    def generate(site)
      site.data['getting_started']['dvp_data']['installTypes'].each do |installTypeKey, installTypeData|

        next unless installTypeData['steps']

        puts "[GS DVP] Generating pages for %s..." % [installTypeKey]

        installTypeData['steps'].each do |stepName, stepData|
          languages = installTypeData['languages'] ? installTypeData['languages'] : ['ru', 'en']
          languages.each do |lang|
            site.pages << DVP_GSPage.new(site, site.data['getting_started']['dvp_data']['global'], installTypeKey, installTypeData, stepName, lang)
          end
        end
      end
    end
  end

  class Stronghold_GSGenerator < Jekyll::Generator
    safe true

    def generate(site)
      site.data['getting_started']['stronghold']['installTypes'].each do |installTypeKey, installTypeData|

        next unless installTypeData['steps']

        puts "[GS Stronghold] Generating for %s..." % [installTypeKey]

        installTypeData['steps'].each do |stepName, stepData|
          languages = installTypeData['languages'] ? installTypeData['languages'] : ['ru', 'en']
          languages.each do |lang|
            site.pages << Stronghold_GSPage.new(site, site.data['getting_started']['stronghold']['global'], installTypeKey, installTypeData, stepName, lang)
          end
        end
      end
    end
  end

end

class DKP_GSPage < Jekyll::Page
  def initialize(site, globalData, installName, installData, stepName, lang)
    @site = site
    @base = site.source
    @lang = lang
    @globalData = globalData
    @installName = installName
    @installData = installData
    @stepName = stepName
    @stepData = installData['steps'][stepName]
    @stepNumber = @stepName.gsub(%r!\D!, "").to_i

    @dir = @dir.sub(%r{^\.$}, '').sub(%r{^/?(ru|en)/}, '').sub(%r{^(ru|en)$}, '').sub(%r{^/}, '') if @dir
    @dir = getFromHash(@globalData, 'step', 'output_dir_template').sub(%r!<LANG>!, @lang).sub(%r!<INSTALL_CODE>!, @installName)
    @name = "#{stepName}.md"

    self.process(@name)
    @path = site.in_source_dir(@base, @dir, @name)
    @path = File.join(@path, "index.html") if url.end_with?("/")

    titleGen = installData['pages_title'][lang][site.data['i18n']['kubernetes-platform'].size, installData['pages_title'][lang].size]

    self.data = {
      'title' => "%s: %s" % [installData['pages_title'][lang], @stepData['name'][lang]],
      'title_gen' => titleGen ? "#{@stepData['name'][lang]}#{titleGen}" : nil,
      'title_main' => "%s" % installData['pages_title'][lang],
      'step_name' => @stepData['name'][lang],
      'layout' => @globalData['layout'],
      'searchable' => false,
      'platform_code' => @installName,
      'platform_type' => @installData['type'],
      'platform_name' => @installData['name'],
      'product_code' => 'kubernetes-platform',
      'sitemap_include' => false,
      'url_prefix' => '/products/kubernetes-platform',
      'gs_data_key' => 'dkp_data',
      'toc' => false,
      'steps' => (installData['steps'].length + 1).to_s,
      'step' => @stepNumber.to_s,
      'lang' => @lang,
      'output' => 'web'
    }

    if @installData['steps'].keys.last != @stepName
      self.data['nextStepName'] = @installData['steps']["step#{(@stepNumber + 1).to_s}"]['name'][lang]
    end

    # TODO Refactor this weird logic
    self.data['ee_only'] = true if @installData['ee_only']
    self.data['ce_only'] = true if @installData['ce_only']
    self.data['se_support'] = true if @installData['se_support']

    self.content = "{% include #{globalData['step']['header']} %}\n\n"

    if @stepData['template']
      include_url = @stepData['template'].gsub(%r!<INSTALL_CODE>!, @installName)
      if @lang == 'ru'
        self.content << "\n{% include #{include_url.sub(%r!\.md$!, '_RU.md').sub(%r!\.html$!, '_ru.html')} %}\n"
      else
        self.content << "\n{% include #{include_url} %}\n"
      end
    end

    self.content << "{% include #{globalData['step']['footer']} %}\n"

    Jekyll::Hooks.trigger :pages, :post_init, self
  end
end

class Stronghold_GSPage < Jekyll::Page
  def initialize(site, globalData, installName, installData, stepName, lang)
    @site = site
    @base = site.source
    @lang = lang
    @globalData = globalData
    @installName = installName
    @installData = installData
    @stepName = stepName
    @stepData = installData['steps'][stepName]
    @stepNumber = @stepName.gsub(%r!\D!, "").to_i

    @dir = @dir.sub(%r{^\.$}, '').sub(%r{^/?(ru|en)/}, '').sub(%r{^(ru|en)$}, '').sub(%r{^/}, '') if @dir
    @dir = getFromHash(@globalData, 'step', 'output_dir_template').sub(%r!<LANG>!, @lang).sub(%r!<INSTALL_CODE>!, @installName)
    @name = "#{stepName}.md"

    self.process(@name)
    @path = site.in_source_dir(@base, @dir, @name)
    @path = File.join(@path, "index.html") if url.end_with?("/")

    titleGen = installData['pages_title'][lang][site.data['i18n']['stronghold'].size, installData['pages_title'][lang].size]

    self.data = {
      'title' => "%s: %s" % [installData['pages_title'][lang], @stepData['name'][lang]],
      'title_gen' => titleGen ? "#{@stepData['name'][lang]}#{titleGen}" : nil,
      'title_main' => "%s" % installData['pages_title'][lang],
      'step_name' => @stepData['name'][lang],
      'layout' => @globalData['layout'],
      'searchable' => false,
      'platform_code' => @installName,
      'platform_type' => @installData['type'],
      'platform_name' => @installData['name'],
      'product_code' => 'stronghold',
      'sitemap_include' => false,
      'url_prefix' => '/products/stronghold',
      'gs_data_key' => 'stronghold',
      'toc' => false,
      'steps' => (installData['steps'].length + 1).to_s,
      'step' => @stepNumber.to_s,
      'lang' => @lang,
      'output' => 'web'
    }

    if @installData['steps'].keys.last != @stepName
      self.data['nextStepName'] = @installData['steps']["step#{(@stepNumber + 1).to_s}"]['name'][lang]
    end

    # TODO Refactor this weird logic
    self.data['ee_only'] = true if @installData['ee_only']
    self.data['ce_only'] = true if @installData['ce_only']
    self.data['se_support'] = true if @installData['se_support']

    self.content = "{% include #{globalData['step']['header']} %}\n\n"

    if @stepData['template']
      include_url = @stepData['template'].gsub(%r!<INSTALL_CODE>!, @installName)
      if @lang == 'ru'
        self.content << "\n{% include #{include_url.sub(%r!\.md$!, '_RU.md').sub(%r!\.html$!, '_ru.html')} %}\n"
      else
        self.content << "\n{% include #{include_url} %}\n"
      end
    end

    self.content << "{% include #{globalData['step']['footer']} %}\n"

    Jekyll::Hooks.trigger :pages, :post_init, self
  end
end

class DVP_GSPage < Jekyll::Page
  def initialize(site, globalData, installName, installData, stepName, lang)
    @site = site
    @base = site.source
    @lang = lang
    @globalData = globalData
    @installName = installName
    @installData = installData
    @stepName = stepName
    @stepData = installData['steps'][stepName]
    @stepNumber = @stepName.gsub(%r!\D!, "").to_i

    @dir = @dir.sub(%r{^\.$}, '').sub(%r{^/?(ru|en)/}, '').sub(%r{^(ru|en)$}, '').sub(%r{^/}, '') if @dir
    @dir = getFromHash(@globalData, 'step', 'output_dir_template').sub(%r!<LANG>!, @lang).sub(%r!<INSTALL_CODE>!, @installName)
    @name = "#{stepName}.md"
    @name = "index.md" if @stepNumber == 1

    self.process(@name)
    @path = site.in_source_dir(@base, @dir, @name)
    @path = File.join(@path, "index.html") if url.end_with?("/")

    titleGen = installData['pages_title'][lang][site.data['i18n']['virtualization-platform'].size, installData['pages_title'][lang].size]

    self.data = {
      'title' => "%s: %s" % [installData['pages_title'][lang], @stepData['name'][lang]],
      'title_gen' => titleGen ? "#{@stepData['name'][lang]}#{titleGen}" : nil,
      'title_main' => "%s" % installData['pages_title'][lang],
      'step_name' => @stepData['name'][lang],
      'layout' => @globalData['layout'],
      'searchable' => false,
      'platform_code' => @installName,
      'platform_type' => @installData['type'],
      'platform_name' => @installData['name'],
      'product_code' => 'virtualization-platform',
      'sitemap_include' => false,
      'searchBageEnabled' => false,
      'url_prefix' => '/products/virtualization-platform',
      'gs_data_key' => 'dvp_data',
      'toc' => false,
      'steps' => (installData['steps'].length).to_s,
      'step' => @stepNumber.to_s,
      'lang' => @lang,
      'output' => 'web'
    }

    if @installData['steps'].keys.last != @stepName
      self.data['nextStepName'] = @installData['steps']["step#{(@stepNumber + 1).to_s}"]['name'][lang]
    end

    self.data['ee_only'] = true if @installData['ee_only']
    self.data['ce_only'] = true if @installData['ce_only']

    self.content = "{% include #{globalData['step']['header']} %}\n\n"

    if @stepData['template']
      include_url = @stepData['template'].gsub(%r!<INSTALL_CODE>!, @installName)
      if @lang == 'ru'
        self.content << "\n{% include #{include_url.sub(%r!\.md$!, '_RU.md').sub(%r!\.html$!, '_ru.html')} %}\n"
      else
        self.content << "\n{% include #{include_url} %}\n"
      end
    end

    self.content << "{% include #{globalData['step']['footer']} %}\n"

    Jekyll::Hooks.trigger :pages, :post_init, self
  end
end

def getFromHash(input, *keys)
  input ? input.dig(*keys) : nil
end
