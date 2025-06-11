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

