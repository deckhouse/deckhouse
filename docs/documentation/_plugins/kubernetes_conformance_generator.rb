module KubernetesConformance
  class Generator < Jekyll::Generator
    safe true
    priority :normal

    JUNIT_FILENAME = 'junit_01.xml'

    def generate(site)
      conformance_dir = File.join(site.source, 'assets', 'conformance')

      site.data['kubernetes_conformance'] = {
        'results' => conformance_results(conformance_dir),
        'readme' => conformance_readme(conformance_dir)
      }
    end

    private

    def conformance_results(conformance_dir)
      return [] unless Dir.exist?(conformance_dir)

      Dir.children(conformance_dir).each_with_object([]) do |version, results|
        next unless version.match?(/\A\d+\.\d+\z/)

        xml_path = File.join(conformance_dir, version, JUNIT_FILENAME)
        next unless File.file?(xml_path)

        results << {
          'version' => version,
          'xml_path' => "/assets/conformance/#{version}/#{JUNIT_FILENAME}"
        }
      end.sort_by { |result| result['version'].split('.').map(&:to_i) }.reverse
    end

    def conformance_readme(conformance_dir)
      readme_path = File.join(conformance_dir, 'README.md')
      return '' unless File.file?(readme_path)

      demote_markdown_headings(File.read(readme_path))
    end

    def demote_markdown_headings(markdown)
      fence_character = nil

      markdown.each_line.map do |line|
        fence = line.match(/\A\s*(`{3,}|~{3,})/)

        if fence_character
          fence_character = nil if line.match?(/\A\s*#{Regexp.escape(fence_character)}{3,}\s*\z/)
          line
        elsif fence
          fence_character = fence[1][0]
          line
        elsif line.match?(/\A\#{1,5}\s/)
          "##{line}"
        else
          line
        end
      end.join
    end
  end
end
