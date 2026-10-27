# Adds patch release changelogs to the Release notes page.
#
# The editorial release notes describe minor versions only (`## Version X.Y`).
# CI writes patch changelogs to CHANGELOG/CHANGELOG-vX.Y.Z.yml; the documentation
# image gets them in the `_changelog` directory (see werf-git-section.inc.yaml).
# The `inject_patch_releases` filter appends a collapsible subsection per patch
# to the end of the section of its minor version.
#
# The logic in ReleaseNotesPatches has no Jekyll dependency and can be run
# from a console.

require "yaml"

module ReleaseNotesPatches
  PATCH_FILE = /\ACHANGELOG-v(\d+)\.(\d+)\.(\d+)\.yml\z/
  MINOR_HEADING = /\A## (?:Version|Версия) (\d+\.\d+)[ \t]*\z/
  H2 = /\A## /
  FENCE = /\A\s*(```|~~~)/
  SECTION_ORDER = %w[features fixes chore].freeze
  LABEL_KEYS = %w[know_before_update features fixes chore].freeze

  # Links to internal resources are not published: the merge request is not
  # needed to read a changelog entry.
  HIDDEN_LINK_TARGET = %r{https?://(?:fox\.flant\.com|github\.com/deckhouse/deckhouse/pull)/[^)\s]*}
  REFERENCE_LINK = /[ \t]*\[#\d+\]\(#{HIDDEN_LINK_TARGET}\)/
  TEXT_LINK = /\[([^\]]*)\]\(#{HIDDEN_LINK_TARGET}\)/
  BARE_LINK = /[ \t]*<?#{HIDDEN_LINK_TARGET}>?/

  module_function

  # Indexes CHANGELOG-vX.Y.Z.yml files in dir.
  # Returns {"X.Y" => [{"version" => "X.Y.Z", "path" => ...}, ...]}, patches in
  # descending order. X.Y.0 is skipped: the editorial release notes describe
  # it. Files are parsed later, only for minor versions present in the release
  # notes: CHANGELOG/ also keeps files of old versions in outdated formats.
  def load_dir(dir)
    return {} unless Dir.exist?(dir)

    patches = Dir.children(dir).filter_map do |name|
      match = PATCH_FILE.match(name)
      raise ArgumentError, "#{File.join(dir, name)}: expected a CHANGELOG-vX.Y.Z.yml file" unless match
      next if match[3].to_i.zero?

      { "minor" => "#{match[1]}.#{match[2]}", "patch" => match[3].to_i,
        "version" => "#{match[1]}.#{match[2]}.#{match[3]}", "path" => File.join(dir, name) }
    end

    patches.group_by { |patch| patch["minor"] }.transform_values do |items|
      items.sort_by { |patch| -patch["patch"] }.map { |patch| patch.slice("version", "path") }
    end
  end

  def load_changes(path)
    validate(YAML.safe_load(File.read(path, encoding: "UTF-8")) || {}, path)
  rescue Psych::Exception => e
    raise ArgumentError, "#{path}: #{e.message}"
  end

  # Checks the structure <module>: {<section>: [{summary, impact?}]}.
  def validate(changes, path)
    raise ArgumentError, "#{path}: expected a map of modules" unless changes.is_a?(Hash)

    changes.each do |mod, sections|
      raise ArgumentError, "#{path}: #{mod}: expected a map of sections" unless sections.is_a?(Hash)

      sections.each do |section, entries|
        raise ArgumentError, "#{path}: #{mod}.#{section}: expected a list of entries" unless entries.is_a?(Array)

        entries.each do |entry|
          unless entry.is_a?(Hash) && entry["summary"].to_s.strip != ""
            raise ArgumentError, "#{path}: #{mod}.#{section}: an entry has no summary"
          end
        end
      end
    end
    changes
  end

  # Returns section labels for lang from the release_notes section of i18n.yml.
  def labels(i18n, lang)
    LABEL_KEYS.to_h do |key|
      label = i18n&.dig(key, lang)
      raise ArgumentError, "i18n.yml: release_notes.#{key}.#{lang} is not set" if label.to_s.empty?

      [key, label]
    end
  end

  # Appends patch subsections to the end of each `## Version X.Y` section.
  def inject(markdown, patches_by_minor, labels)
    result = []
    current_minor = nil
    in_fence = false

    flush = lambda do
      patches = current_minor && patches_by_minor[current_minor]
      if patches && !patches.empty?
        result.pop while result.last&.strip == ""
        result << "" << render_minor(patches, labels) << ""
      end
      current_minor = nil
    end

    markdown.to_s.each_line(chomp: true) do |line|
      in_fence = !in_fence if FENCE.match?(line)

      if !in_fence && H2.match?(line)
        flush.call
        current_minor = MINOR_HEADING.match(line)&.[](1)
      end
      result << line
    end
    flush.call

    result.join("\n") + "\n"
  end

  def render_minor(patches, labels)
    blocks = patches.map { |patch| render_patch(patch["version"], load_changes(patch["path"]), labels) }
    ['<div class="collapsible-sections" markdown="1">', "", *blocks, "</div>"].join("\n")
  end

  def render_patch(version, changes, labels)
    lines = ["### #{version} {##{anchor(version)}}", "", '<div markdown="1">', ""]

    impacts = changes.values.flat_map(&:values).flatten
                     .map { |entry| clean(entry["impact"]) }.reject(&:empty?).uniq.sort
    lines.concat(render_section(labels["know_before_update"], impacts.map { |impact| list_item(impact) }))

    sections_in_order(changes).each do |section|
      items = changes.sort.flat_map do |mod, sections|
        Array(sections[section]).map { |entry| render_entry(mod, entry) }
      end
      lines.concat(render_section(labels.fetch(section) { humanize(section) }, items))
    end

    lines << "</div>" << ""
    lines.join("\n")
  end

  def render_section(title, items)
    return [] if items.empty?

    ["**#{title}**", "", *items, ""]
  end

  def render_entry(mod, entry)
    text = "**[#{mod}]** #{clean(entry['summary'])}"
    impact = clean(entry["impact"])
    text += "  \n#{impact}" unless impact.empty?
    list_item(text)
  end

  def list_item(text)
    "- " + text.gsub("\n", "\n  ")
  end

  # Known sections first, others after them alphabetically.
  def sections_in_order(changes)
    present = changes.values.flat_map(&:keys).uniq
    (SECTION_ORDER & present) + (present - SECTION_ORDER).sort
  end

  def clean(text)
    text.to_s.gsub(REFERENCE_LINK, "").gsub(TEXT_LINK, '\1').gsub(BARE_LINK, "").strip
  end

  def anchor(version)
    "v#{version.tr('.', '-')}"
  end

  def humanize(section)
    section.to_s.tr("_", " ").capitalize
  end
end

if defined?(Jekyll)
  module Jekyll
    class PatchChangelogsGenerator < Generator
      safe true
      priority :high

      def generate(site)
        site.data["patch_changelogs"] = ReleaseNotesPatches.load_dir(File.join(site.source, "_changelog"))
      end
    end

    module ReleaseNotesPatchesFilter
      def inject_patch_releases(markdown, lang)
        site = @context.registers[:site]
        labels = ReleaseNotesPatches.labels(site.data.dig("i18n", "release_notes"), lang || "en")
        ReleaseNotesPatches.inject(markdown, site.data["patch_changelogs"] || {}, labels)
      end
    end
  end

  Liquid::Template.register_filter(Jekyll::ReleaseNotesPatchesFilter)
end
