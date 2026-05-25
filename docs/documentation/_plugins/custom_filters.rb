require_relative "utils"

module Jekyll
  module CustomFilters
    STRIP_HTML_BLOCKS       = Regexp.union(
      %r{<script.*?</script>}m,
      /<!--.*?-->/m,
      %r{<style.*?</style>}m
    )
    STRIP_MD_TABLES       = Regexp.union(
      %r{\|\ ?[:+-= ]+\ ?\|},
      %r#[:+-= ]{4,}#,
      %r{\|\|+}
    )
    STRIP_LIQUID_TAGS       = Regexp.union(
      /\{{.*?}}/m,
      /\{%.*?%}/m
    )
    STRIP_HTML_TAGS = /<.*?>/m

    def true_relative_url(path)
        getTrueRelativeUrl(path)
    end

    def endswith(text, query)
      return text.end_with? query
    end

    def camel_to_snake_case(text)
      return text.to_s.gsub(/([A-Z]+)([A-Z][a-z])/,'\1_\2').
                         gsub(/([a-z\d])([A-Z])/,'\1_\2').
                         tr("-", "_").downcase
    end

    def normalizeSearchContent(text)
      return text.to_s.gsub(STRIP_HTML_BLOCKS, ' ').
                       gsub(STRIP_HTML_TAGS, ' ').
                       gsub(STRIP_MD_TABLES,' ').
                       gsub(STRIP_LIQUID_TAGS, ' ').
                       gsub(/\n/,' ').
                       gsub(/\s\s+/,' ').strip
    end

    def normalizeAlertContent(text)
      return text.to_s.gsub(/\n/m,'#RET').
              gsub(/{{\s*\$labels\.annotation\s*\}\}/m,'ANNOTATION_NAME').
              gsub(/{{\s*\$labels\.api_host\s*\}\}/m,'API_HOST').
              gsub(/{{\s*\$labels\.cluster_id\s*\}\}/m,'CLUSTER_ID').
              gsub(/{{\s*\$labels\.cni\s*\}\}/m,'CNI_NAME').
              gsub(/{{\s*\$labels\.component\s*\}\}/m,'COMPONENT_NAME').
              gsub(/{{\s*\$labels\.component_id\s*\}\}/m,'COMPONENT_ID').
              gsub(/{{\s*\$labels\.component_type\s*\}\}/m,'COMPONENT_TYPE').
              gsub(/{{\s*\$labels\.container\s*\}\}/m,'CONTAINER_NAME').
              gsub(/{{\s*\$labels\.container_runtime_version\s*\}\}/m,'Containerd X.XX.XXX').
              gsub(/{{\s*\$labels\.controller\s*\}\}/m,'CONTROLLER_NAME').
              gsub(/{{\s*\$labels\.controller_namespace\s*\}\}/m,'CONTROLLER_NAMESPACE').
              gsub(/{{\s*\$labels\.controller_pod\s*\}\}/m,'CONTROLLER_POD_NAME').
              gsub(/{{\s*\$labels\.cronjob\s*\}\}/m,'CRONJOB').
              gsub(/{{\s*\$labels\.daemonset\s*\}\}/m,'DAEMONSET_NAME').
              gsub(/{{\s*\$labels\.dataplane_pod\s*\}\}/m,'POD_NAME').
              gsub(/{{\s*\$labels\.deployment\s*\}\}/m,'DEPLOYMENT_NAME').
              gsub(/{{\s*\$labels\.desired_full_version\s*\}\}/m,'DESIRED_VERSION').
              gsub(/{{\s*\$labels\.desired_revision\s*\}\}/m,'DESIRED_VISION').
              gsub(/{{\s*\$labels\.destination_node\s*\}\}/m,'NODE_NAME').
              gsub(/{{\s*\$labels\.device\s*\}\}/m,'NODE_DISK_NAME').
              gsub(/{{\s*\$labels\.egressgateway\s*\}\}/m,'EGRESSGATEWAY').
              gsub(/{{\s*\$labels\.endpoint\s*\}\}/m,'ENDPOINT_NAME').
              gsub(/{{\s*\$labels\.error_type\s*\}\}/m,'ERROR_TYPE').
              gsub(/{{\s*\$labels\.exported_namespace\s*\}\}/m,'NAMESPACE').
              gsub(/{{\s*\$labels\.federation_name\s*\}\}/m,'FEDERATION_NAME').
              gsub(/{{\s*\$labels\.full_version\s*\}\}/m,'VERSION_NUMBER').
              gsub(/{{\s*\$labels\.hook\s*\}\}/m,'HOOK_NAME').
              gsub(/{{\s*\$labels\.host\s*\}\}/m,'HOST_NAME').
              gsub(/{{\s*\$labels\.id\s*\}\}/m,'ID').
              gsub(/{{\s*\$labels\.image\s*\}\}/m,'IMAGE_NAME').
              gsub(/{{\s*\$labels\.ingress\s*\}\}/m,'INGRESS').
              gsub(/{{\s*\$labels\.instance\s*\}\}/m,'INSTANCE_NAME').
              gsub(/{{\s*\$labels\.istio_version\s*\}\}/m,'VERSION_NUMBER').
              gsub(/{{\s*\$labels\.istiod\s*\}\}/m,'INSTANCE_NAME').
              gsub(/{{\s*\$labels\.job\s*\}\}/m,'JOB_NAME').
              gsub(/{{\s*\$labels\.job_name\s*\}\}/m,'JOB_NAME').
              gsub(/{{\s*\$labels\.k8s_version\s*\}\}/m,'VERSION_NUMBER').
              gsub(/{{\s*\$labels\.kubelet_version\s*\}\}/m,'VERSION_NUMBER').
              gsub(/{{\s*\$labels\.label_istio_io_rev\s*\}\}/m,'ISTIO_REVISION_LABEL').
              gsub(/{{\s*\$labels\.location\s*\}\}/m,'LOCATION').
              gsub(/{{\s*\$labels\.machine_deployment_name\s*\}\}/m,'MACHINE_DEPLOYMENT_NAME').
              gsub(/{{\s*\$labels\.map_name\s*\}\}/m,'MAP_NAME').
              gsub(/{{\s*\$labels\.message\s*\}\}/m,'MESSAGE_CONTENTS').
              gsub(/{{\s*\$labels\.module\s*\}\}/m,'MODULE_NAME').
              gsub(/{{\s*\$labels\.module_name\s*\}\}/m,'MODULE_NAME').
              gsub(/{{\s*\$labels\.moduleName\s*\}\}/m,'MODULE_NAME').
              gsub(/{{\s*\$labels\.module_release\s*\}\}/m,'MODULE_RELEASE').
              gsub(/{{\s*\$labels\.mountpoint\s*\}\}/m,'MOUNTPOINT').
              gsub(/{{\s*\$labels\.multicluster_name\s*\}\}/m,'MULTICLUSTER_NAME').
              gsub(/{{\s*\$labels\.namespace\s*\}\}/m,'NAMESPACE').
              gsub(/{{\s*\$labels\.name\s*\}\}/m,'NAME').
              gsub(/{{\s*\$labels\.node\s*\}\}/m,'NODE_NAME').
              gsub(/{{\s*\$labels\.node_group\s*\}\}/m,'NODE_GROUP_NAME').
              gsub(/{{\s*\$labels\.node_group_name\s*\}\}/m,'NODE_GROUP_NAME').
              gsub(/{{\s*\$labels\.owner_name\s*\}\}/m,'CRONJOB').
              gsub(/{{\s*\$labels\.path\s*\}\}/m,'PATH').
              gsub(/{{\s*\$labels\.peer\s*\}\}/m,'PEER').
              gsub(/{{\s*\$labels\.persistentvolumeclaim\s*\}\}/m,'PVC_NAME').
              gsub(/{{\s*\$labels\.pod\s*\}\}/m,'POD_NAME').
              gsub(/{{\s*\$labels\.pod_name\s*\}\}/m,'POD_NAME').
              gsub(/{{\s*\$labels\.port\s*\}\}/m,'PORT_NUMBER').
              gsub(/{{\s*\$labels\.phase\s*\}\}/m,'STATUS').
              gsub(/{{\s*\$labels\.queue\s*\}\}/m,'QUEUE_NAME').
              gsub(/{{\s*\$labels\.resource_name\s*\}\}/m,'RESOURCE_NAME').
              gsub(/{{\s*\$labels\.revision\s*\}\}/m,'REVISION_NUMBER').
              gsub(/{{\s*\$labels\.scheme\s*\}\}/m,'SCHEME').
              gsub(/{{\s*\$labels\.secret_name\s*\}\}/m,'SECRET_NAME').
              gsub(/{{\s*\$labels\.secret_namespace\s*\}\}/m,'SECRET_NAMESPACE').
              gsub(/{{\s*\$labels\.service\s*\}\}/m,'SERVICE_NAME').
              gsub(/{{\s*\$labels\.service_port\s*\}\}/m,'SERVICE_PORT').
              gsub(/{{\s*\$labels\.stage\s*\}\}/m,'STAGE_NAME').
              gsub(/{{\s*\$labels\.statefulset\s*\}\}/m,'STATEFULSET').
              gsub(/{{\s*\$labels\.status\s*\}\}/m,'STATUS_REFERENCE').
              gsub(/{{\s*\$labels\.storageclass\s*\}\}/m,'STORAGECLASS_NAME').
              gsub(/{{\s*\$labels\.type\s*\}\}/m,'ERROR_TYPE').
              gsub(/{{\s*\$labels\.updatePolicy\s*\}\}/m,'UPDATE_POLICY_NAME').
              gsub(/{{\s*\$labels\.vhost\s*\}\}/m,'VHOST/').
              gsub(/{{\s*\$labels\.version\s*\}\}/m,'VERSION_NUMBER').
              gsub(/{{\s*\$result\.Labels\.pod\s*\}\}/m,'POD_NAME').
              gsub(/{{\s*[$\.][Vv]alue\s*\}\}/m,'VALUE').
              gsub(/\{\{.*?\}\}/m,'XXX').
              gsub(/<([a-zA-Z0-9]+[^>]*)>/,'\1').
              gsub(/#RET/m,"\n").strip
    end

    def startswith(text, query)
      return text.start_with? query if text
    end

    # get_lang_field_or_raise_error filter returns a field from argument hash
    # returns nil if hash is empty
    # returns hash[page.lang] if hash has the field
    # returns hash["all"] if hash has the field
    # otherwise, raise an error
    def get_lang_field_or_raise_error(hash)
        if !(hash == nil or hash.instance_of? Hash)
            raise "get_lang_field_or_raise_error filter failed: unexpected argument '#{hash}'"
        end

        if hash == nil or hash.length == 0
            return
        end

        lang = @context.registers[:page]["lang"]
        if hash.has_key?(lang)
            return hash[lang]
        elsif hash.has_key?("all")
            return hash["all"]
        else
            raise "get_lang_field_or_raise_error filter failed: the argument '#{hash}' does not have '#{lang}' or 'all' field"
        end
    end
  end
end

Liquid::Template.register_filter(Jekyll::CustomFilters)
