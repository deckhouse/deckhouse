module CRDGenerator
  class CRDGenerator < Jekyll::Generator
    safe true

    def generate(site)
      site.data['schemas']['virtualization-platform']['crds'].sort.each do |crdKey, crdData|
        next unless crdData["spec"]["names"]["kind"]
        puts "Processing CRD %s..." % [crdData["spec"]["names"]["kind"]]

        languages = ['ru', 'en']
        languages.each do |lang|
          site.pages << CRDPage.new(site, crdKey, crdData, lang)
        end
      end
    end
  end
end

class CRDPage < Jekyll::Page
  def initialize(site, crdKey, crdData, lang)
    @site = site
    @base = site.source
    @lang = lang
    @crdData = crdData
    @crdKey = crdKey

    @fileName =  "%s.html" % crdData["spec"]["names"]["kind"].downcase

    @path = "virtualization-platform/reference/cr/#{@fileName}"

    self.data = {
      'title' => "Custom resource %s" % @crdData["spec"]["names"]["kind"],
      'searchable' => false,
      'permalink' => "%s/%s" % [ @lang, @path ],
      'layout' => 'page',
      'lang' => @lang,
      'sidebar' => 'virtualization-platform',
      'sitemap_include' => false
    }

    self.content = %q({{ site.data.schemas.virtualization-platform.crds['%s'] | format_crd: "" }}) % @crdKey

    self.process(@path)

    Jekyll::Hooks.trigger :pages, :post_init, self
  end
end
