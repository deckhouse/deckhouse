module JekyllLiquidBlockUtils
  def dedent(text)
    lines = text.split("\n")
    # Exclude HTML lines (already-rendered inner tags) from indent calculation
    # so their col-0 position doesn't force min_indent to 0.
    non_empty = lines.select { |l| l =~ /\S/ && !l.lstrip.start_with?('<') }
    non_empty = lines.select { |l| l =~ /\S/ } if non_empty.empty?
    return text if non_empty.empty?
    min_indent = non_empty.map { |l| l.match(/^(\s*)/)[1].length }.min
    return text if min_indent == 0
    lines.map { |l| l.length >= min_indent ? l[min_indent..] : l }.join("\n")
  end

  # Collapse all newlines outside <pre> blocks and replace all newlines inside
  # <pre> blocks with &#10; entities. Kramdown's block HTML parser breaks the
  # HTML structure on any bare newline inside div markdown="0", so the output
  # must be completely newline-free. &#10; is rendered identically by browsers.
  def collapse_inter_block_newlines(html)
    parts = html.split(/(<pre\b[^>]*>.*?<\/pre>)/m)
    parts.map.with_index do |part, i|
      if i.even?
        part.strip.gsub(/\n+/, ' ')
      else
        # Capture groups from the outer split are stable — no inner gsub here,
        # so $~ is not clobbered. Replace every newline so the whole plugin
        # output is a single line and Kramdown cannot break the block.
        part.gsub("\n", '&#10;')
      end
    end.join
  end
end

def getTrueRelativeUrl(path)
    if !path.instance_of? String
        return "unexpected argument #{path}"
        raise "true_relative_url filter failed: unexpected argument #{path}"
    end

    # remove first slash if exist
    page_path_relative = @context.registers[:page]["url"].gsub(%r!^/!, "")
    page_depth = page_path_relative.scan(%r!/!).count - 1
    prefix = ""
    page_depth.times{ prefix = prefix + "../" }
    prefix + path.sub(%r!^/!, "./")
end

def compare_crd_by_name(crd_a, crd_b)
  return crd_a[0] <=> crd_b[0] if ! (crd_a[1].dig("spec", "names", "kind") && crd_b[1].dig("spec", "names", "kind"))
  crd_a[1]["spec"]["names"]["kind"] <=> crd_b[1]["spec"]["names"]["kind"]
end

def sidebar_validate_item(item)
  if !item.has_key?('title')
    puts "[DEBUG] Sidebar item: #{item}"
    raise "Sidebar item must have a title."
  elsif !(item['title'].has_key?('en') or item['title'].has_key?('ru') )
    puts "[DEBUG] Sidebar item: #{item}"
    raise "Sidebar item doesn't have a valid title."
  elsif !(item.has_key?('url') or item.has_key?('external_url') or item.has_key?('folders'))
    puts "[DEBUG] Sidebar item: #{item}"
    raise "Sidebar item doesn't have url, external_url or folders parameters."
  end
end
