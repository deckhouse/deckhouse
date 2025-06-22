require_relative "render-jsonschema"

module ResourceGenerator
  class ResourceGenerator < Jekyll::Generator
    safe true

    def generate(site)

      return
      languages = ['ru', 'en']
      productCode = 'kubernetes-platform'
      sidebar = 'kubernetes-platform'
      # Modules
      site.data['schemas']['modules'].each do |mcName, mcItems|

        next if mcName == ("crds")

        mcItems.each do |item, value|
          next if item.start_with?("doc-ru-")

          if site.data['schemas']['modules'][mcName].has_key?("doc-ru-#{item}")
            site.data['schemas']['modules'][mcName]['config-values']['i18n'] = {} if site.data['schemas']['modules'][mcName]['config-values']['i18n'].nil?
            site.data['schemas']['modules'][mcName]['config-values']['i18n']['ru'] = site.data['schemas']['modules'][mcName]["doc-ru-#{item}"]
            site.data['schemas']['modules'][mcName].delete("doc-ru-#{item}")
          end
        end
      end

      site.data['schemas']['modules'].sort.each do |mcName, mcItems|

        next if mcName == ("crds")
        puts "Generating page for MC %s..." % [mcName]
        languages.each do |lang|
          site.pages << MCPage.new(site, productCode, sidebar, mcName, mcItems['config-values'], lang)
        end

      end


      #  CRDs
      site.data['schemas']['crds-list'] = []
      site.data['schemas']['crds'].each do |crdKey, crdData|
        next if crdKey.start_with?("doc-ru-")

        if site.data['schemas']['crds'].has_key?("doc-ru-#{crdKey}")
          crdData['i18n'] = {} if crdData['i18n'].nil?
          crdData['i18n']['ru'] = site.data['schemas']['crds']["doc-ru-#{crdKey}"]
          site.data['schemas']['crds'].delete("doc-ru-#{crdKey}")
        end

        if crdData["apiVersions"] and crdData["kind"] then
          puts "Generating page for CRD %s..." % [crdData["kind"]]
          site.data['schemas']['crds-list'].push(crdData["kind"])
          languages.each do |lang|
            site.pages << ResourcePage.new(site, productCode, sidebar, crdKey, crdData, lang, "resource")
          end
        elsif crdData["spec"] and crdData["spec"]["names"]
          next unless crdData["spec"]["names"]["kind"]
          puts "Generating page for CRD %s..." % [crdData["spec"]["names"]["kind"]]

          site.data['schemas']['crds-list'].push(crdData["spec"]["names"]["kind"])
          languages.each do |lang|

            site.pages << ResourcePage.new(site, productCode, sidebar, crdKey, crdData, lang)
          end
        end
        # sort CRD array
        site.data['schemas']['crds-list'] = site.data['schemas']['crds-list'].sort
      end
    end
  end
end

class ResourcePage < Jekyll::Page
  def initialize(site, productCode, sidebar, crdKey, crdData, lang, type="crd")
    @site = site
    @base = site.source
    @lang = lang
    @productCode = productCode
    @sidebar = sidebar
    @sidebar_group_page = "/reference/cr/" if ( type == "crd" || type == "resource" )
    @crdData = crdData
    @crdKey = crdKey
    @fileName = ""
    @kind = ""

    type == "crd" ? @kind = crdData["spec"]["names"]["kind"] : @kind = crdData["kind"]
    @fileName =  "index.html"
    #@fileName =  "#{@kind.downcase}"

    @path = "reference/cr/#{@kind.downcase}/#{@fileName}"
    self.process(@path)

    self.data = {
      'title' => "Custom resource #{@kind}",
      'searchable' => false,
      'permalink' => "%s/%s" % [ @lang, @path ],
      'url' => "%s/%s" % [ @lang, @path ],
      'layout' => 'page',
      'lang' => @lang,
      'name' => @fileName,
      'sidebar' => @sidebar,
      'product_code' => @productCode,
      'sidebar_group_page' => @sidebar_group_page,
      'search_bage_enabled' => true,
      'sitemap_include' => false
    }

    @JSONSchema = JSONSchemaRenderer::JSONSchemaRenderer.new()

    self.content = ""
    if type == "crd" then
      _renderedContent = @JSONSchema.format_crd(site, self.data ,site.data['schemas']['crds'][@crdKey], @crdKey)
    else
      _renderedContent = @JSONSchema.format_cluster_configuration(site, self.data, site.data['schemas']['crds'][@crdKey])
    end
    if _renderedContent.nil?
      self.content = @site.data['i18n']['common']['crd_has_no_parameters'][@lang]
    else
      self.content = _renderedContent
    end

    Jekyll::Hooks.trigger :pages, :post_init, self
  end
end

class MCPage < Jekyll::Page
  def initialize(site, productCode, sidebar, mcKey, mcData, lang)
    @site = site
    @base = site.source
    @lang = lang
    @productCode = productCode
    @sidebar = sidebar
    @sidebar_group_page = "/reference/mc/"
    @mcData = mcData
    @mcKey = mcKey
    @fileName = ""
    @kind = mcKey

    @fileName =  "index.html"
    #@fileName =  "#{@kind.downcase}"

    @path = "reference/mc/#{@kind.downcase}/#{@fileName}"
    self.process(@path)

    self.data = {
      'title' => "Module config #{@kind}",
      'searchable' => false,
      'permalink' => "%s/%s" % [ @lang, @path ],
      'url' => "%s/%s" % [ @lang, @path ],
      'layout' => 'page',
      'lang' => @lang,
      'name' => @fileName,
      'sidebar' => @sidebar,
      'product_code' => @productCode,
      'sidebar_group_page' => @sidebar_group_page,
      'search_bage_enabled' => true,
      'sitemap_include' => false
    }

    @JSONSchema = JSONSchemaRenderer::JSONSchemaRenderer.new()

    self.content = ""
      _renderedContent = @JSONSchema.format_module_configuration(site, self.data, mcData, mcKey )
    if _renderedContent.nil?
      self.content = @site.data['i18n']['common']['crd_has_no_parameters'][@lang]
    else
      self.content = _renderedContent
    end

    Jekyll::Hooks.trigger :pages, :post_init, self
  end
end
