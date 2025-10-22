require_relative "render-jsonschema"

module Jekyll
  module Openapi

    @@JSONSchema = JSONSchemaRenderer::JSONSchemaRenderer.new()

    def format_crd(input, moduleName = "")
        return if input.nil? || input.empty?

        @@JSONSchema.format_crd(@context.registers[:site], @context.registers[:page], input, moduleName)
    end

    def format_cluster_configuration(input, moduleName = "")
        return if input.nil? || input.empty? || input["kind"].nil? || input["kind"].empty?

        puts "Rendering #{input["kind"]} (#{@context.registers[:page]['lang']})"
        @@JSONSchema.format_cluster_configuration(@context.registers[:site], @context.registers[:page], input, moduleName)
    end

    def format_module_configuration(input, moduleName = "")
        return if input.nil? || input.empty? || input["properties"].nil?|| input["properties"].empty?

        puts "Rendering ModuleConfig for #{moduleName} (#{@context.registers[:page]['lang']})"
        @@JSONSchema.format_configuration(@context.registers[:site], @context.registers[:page], input, moduleName, true)
    end
  end
end

Liquid::Template.register_filter(Jekyll::Openapi)
