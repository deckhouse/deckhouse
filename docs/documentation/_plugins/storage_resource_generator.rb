require_relative "render-jsonschema"

module ResourceGenerator
  class StorageResourceGenerator < Jekyll::Generator
    safe true

    def generate(site)
      site.data['storage'].each do |mcName, mcItems|

        next if mcName == ("crds")

        mcItems.each do |item, value|
          next if item.start_with?("doc-ru-")
          site.data['schemas'][mcName] = {}
          site.data['schemas'][mcName]['config-values'] = value
          if mcItems.has_key?("doc-ru-#{item}")
            site.data['schemas'][mcName]['config-values']['i18n'] = {} if site.data['schemas'][mcName]['config-values']['i18n'].nil?
            site.data['schemas'][mcName]['config-values']['i18n']['ru'] = mcItems["doc-ru-#{item}"]
          end
        end
      end


      site.data['storage']['crds'].each do |crdKey, crdData|
        next if crdKey.start_with?("doc-ru-")
        languages = ['ru', 'en']

        if site.data['storage']['crds'].has_key?("doc-ru-#{crdKey}")
          crdData['i18n'] = {} if crdData['i18n'].nil?
          crdData['i18n']['ru'] = site.data['storage']['crds']["doc-ru-#{crdKey}"]
        end

        if crdData["apiVersions"] and crdData["kind"] then
          puts "[STORAGE] Generating page for CRD %s..." % [crdData["kind"]]
          languages.each do |lang|
            site.pages << StorageResourcePage.new(site, crdKey, crdData, lang, "resource")
          end
        elsif crdData["spec"] and crdData["spec"]["names"]
          next unless crdData["spec"]["names"]["kind"]
          puts "[STORAGE] Generating page for CRD %s..." % [crdData["spec"]["names"]["kind"]]

          languages.each do |lang|

            site.pages << StorageResourcePage.new(site, crdKey, crdData, lang)
          end
        end
      end
    end
  end
end

class StorageResourcePage < Jekyll::Page
  def initialize(site, crdKey, crdData, lang, type="crd")
    @site = site
    @base = site.source
    @lang = lang
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
      'sidebar' => 'main',
      'product_code' => 'kubernetes-platform',
      'search_bage_enabled' => true,
      'sitemap_include' => false
    }

    @JSONSchema = JSONSchemaRenderer::JSONSchemaRenderer.new()

    self.content = ""
    if type == "crd" then
      _renderedContent = @JSONSchema.format_crd(site, self.data ,site.data['storage']['crds'][@crdKey], @crdKey)
    else
      _renderedContent = @JSONSchema.format_cluster_configuration(site, self.data, site.data['storage']['crds'][@crdKey])
    end
    if _renderedContent.nil?
      self.content = @site.data['i18n']['common']['crd_has_no_parameters'][@lang]
    else
      self.content = _renderedContent
    end

    Jekyll::Hooks.trigger :pages, :post_init, self
  end
end
