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

