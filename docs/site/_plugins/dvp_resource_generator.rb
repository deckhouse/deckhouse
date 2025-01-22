require_relative "render-jsonschema"

module ResourceGenerator
  class ResourceGenerator < Jekyll::Generator
    safe true

    def generate(site)
      site.data['schemas']['virtualization-platform']['crds'].each do |crdKey, crdData|
        languages = ['ru', 'en']


        if crdData["apiVersions"] and crdData["kind"] then
          puts "Generating page for CRD %s..." % [crdData["kind"]]
          languages.each do |lang|
            site.pages << ResourcePage.new(site, crdKey, crdData, lang, "resource")
          end
        elsif crdData["spec"] and crdData["spec"]["names"]
          next unless crdData["spec"]["names"]["kind"]
          puts "Generating page for CRD %s..." % [crdData["spec"]["names"]["kind"]]

          languages.each do |lang|

            site.pages << ResourcePage.new(site, crdKey, crdData, lang)
          end
        end
      end
    end
  end
end

class ResourcePage < Jekyll::Page
  def initialize(site, crdKey, crdData, lang, type="crd")
    @site = site
    @base = site.source
    @lang = lang
    @crdData = crdData
    @crdKey = crdKey
    @fileName = ""
    @kind = ""

    type == "crd" ? @kind = crdData["spec"]["names"]["kind"] : @kind = crdData["kind"]
    @fileName =  "#{@kind.downcase}.html"

    @path = "virtualization-platform/reference/cr/#{@fileName}"
    self.process(@path)

    self.data = {
      'title' => "Custom resource #{@kind}",
      'searchable' => false,
      'permalink' => "%s/%s" % [ @lang, @path ],
      'layout' => 'page',
      'lang' => @lang,
      'sidebar' => 'virtualization-platform',
      'product_code' => 'virtualization-platform',
      'search_bage_enabled' => true,
      'sitemap_include' => false
    }

    @JSONSchema = JSONSchemaRenderer::JSONSchemaRenderer.new()

    self.content = ""
    if type == "crd" then
      _renderedContent = @JSONSchema.format_crd(site, self.data ,site.data['schemas']['virtualization-platform']['crds'][@crdKey], @crdKey)
    else
      _renderedContent = @JSONSchema.format_cluster_configuration(site, self.data, site.data['schemas']['virtualization-platform']['crds'][@crdKey])
    end
    if _renderedContent.nil?
      self.content = @site.data['i18n']['common']['crd_has_no_parameters'][@lang]
    else
      self.content = _renderedContent
    end

    Jekyll::Hooks.trigger :pages, :post_init, self
  end
end
