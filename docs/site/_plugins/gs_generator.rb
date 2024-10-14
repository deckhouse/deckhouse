module GSGenerator
  class GSGenerator < Jekyll::Generator
    safe true

    def generate(site)
      site.data['getting_started']['data']['installTypes'].each do |installTypeKey, installTypeData|

        next unless installTypeData['steps']

        puts "Processing %s... (%s)" % [installTypeKey, installTypeData['name']]

        installTypeData['steps'].each do |stepName, stepData|
          languages = installTypeData['languages'] ? installTypeData['languages'] : ['ru', 'en']
          languages.each do |lang|
            site.pages << GSPage.new(site, site.data['getting_started']['data']['global'], installTypeKey, installTypeData, stepName, lang)
          end
        end
      end
    end
  end
end

class GSPage < Jekyll::Page
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

    self.data = {
      'title' => "%s: %s" % [installData['pages_title'][lang], @stepData['name'][lang]],
      'title_main' => "%s" % installData['pages_title'][lang],
      'step_name' => @stepData['name'][lang],
      'layout' => @globalData['layout'],
      'searchable' => false,
      'platform_code' => @installName,
      'platform_type' => @installData['type'],
      'platform_name' => @installData['name'],
      'toc' => false,
      'steps' => (installData['steps'].length + 1).to_s,
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

  def getFromHash(input, *keys)
    input ? input.dig(*keys) : nil
  end
end
