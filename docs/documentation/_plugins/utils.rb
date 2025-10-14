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
