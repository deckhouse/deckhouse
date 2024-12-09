module ResourceGenerator
  class ResourceGenerator < Jekyll::Generator
    safe true

    def generate(site)
      ['virtualization-platform', 'stronghold'].each do |productUrlPath|
        if site.data['schemas'][productUrlPath] then
          site.data['schemas'][productUrlPath]['crds'].each do |crdKey, crdData|
            languages = ['ru', 'en']

            if crdData["apiVersions"] and crdData["kind"] then
              puts "(%s) Processing %s..." % [productUrlPath, crdData["kind"]]
              languages.each do |lang|
                site.pages << ResourcePage.new(site, productUrlPath, crdKey, crdData, lang, "resource")
              end
            elsif crdData["spec"] and crdData["spec"]["names"]
              next unless crdData["spec"]["names"]["kind"]
              puts "(%s) Processing CRD %s..." % [productUrlPath, crdData["spec"]["names"]["kind"]]

              languages.each do |lang|
                site.pages << ResourcePage.new(site, productUrlPath, crdKey, crdData, lang)
              end
            end
          end
        end
      end
    end
  end
end

class ResourcePage < Jekyll::Page
  def initialize(site, productUrlPath, crdKey, crdData, lang, type="crd")
    @site = site
    @productUrlPath = productUrlPath
    @base = site.source
    @lang = lang
    @crdData = crdData
    @crdKey = crdKey
    @fileName = ""
    @kind = ""


    type == "crd" ? @kind = crdData["spec"]["names"]["kind"] : @kind = crdData["kind"]
    @fileName =  "#{@kind.downcase}.html"

    @path = "#{@productUrlPath}/reference/cr/#{@fileName}"

    self.data = {
      'title' => "Custom resource #{@kind}",
      'searchable' => false,
      'permalink' => "%s/%s" % [ @lang, @path ],
      'layout' => 'page',
      'lang' => @lang,
      'sidebar' => @productUrlPath,
      'product_code' => @productUrlPath,
      'search_bage_enabled' => true,
      'sitemap_include' => false
    }

    if type == "crd" then
      self.content = %q({{ site.data.schemas.%s.crds['%s'] | format_crd: "" }}) % [@productUrlPath, @crdKey]
    else
      self.content = %q({{ site.data.schemas.%s.crds['%s'] | format_cluster_configuration }}) % [@productUrlPath, @crdKey]
    end

    self.process(@path)

    Jekyll::Hooks.trigger :pages, :post_init, self
  end
end
