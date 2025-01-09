module ResourceGenerator
  class ResourceGenerator < Jekyll::Generator
    safe true

    def generate(site)
      return unless site.data['schemas'] and site.data['schemas']['virtualization-platform'] and site.data['schemas']['virtualization-platform']['crds']
      site.data['schemas']['virtualization-platform']['crds'].each do |crdKey, crdData|
        languages = ['ru', 'en']

        if crdData["apiVersions"] and crdData["kind"] then
          puts "Processing %s..." % [crdData["kind"]]
          languages.each do |lang|
            site.pages << ResourcePage.new(site, crdKey, crdData, lang, "resource")
          end
        elsif crdData["spec"] and crdData["spec"]["names"]
          next unless crdData["spec"]["names"]["kind"]
          puts "Processing CRD %s..." % [crdData["spec"]["names"]["kind"]]

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

    if type == "crd" then
      self.content = %q({{ site.data.schemas.virtualization-platform.crds['%s'] | format_crd: "" }}) % @crdKey
    else
      self.content = %q({{ site.data.schemas.virtualization-platform.crds['%s'] | format_cluster_configuration }}) % @crdKey
    end

    self.process(@path)

    Jekyll::Hooks.trigger :pages, :post_init, self
  end
end
