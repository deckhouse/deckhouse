module Jekyll
  module Openapi

    def format_key_name(name)
      "<code class=\"highlighter-rouge\">#{name}</code>"
    end

    def format_type(first_type, second_type)
        result =    case first_type
                      when "array" then "массив"
                      when "object" then "объект"
                      when "integer" then "целочисленный"
                      when "string" then "строка"
                      when "boolean" then "булевый"
                      else first_type
                    end
        if second_type
            result += ' ' + case second_type
                              when "array" then "массивов"
                              when "object" then "объектов"
                              when "integer" then "целых чисел"
                              when "string" then "строк"
                              when "boolean" then "булевых значений"
                              else "of #{second_type}"
                            end
        end
        result
    end

    def format_attribute(name, attributes, parent)
        result = Array.new()
        converter = Jekyll::Converters::Markdown::KramdownParser.new(Jekyll.configuration())
        result.push(converter.convert(attributes['description'])) if attributes['description']

        result.push(converter.convert("**По умолчанию:** `#{attributes['default'].to_json}`")) if attributes.has_key?('default')

        if attributes['minimum'] || attributes['maximum']
            range = '**Допустимые значения:** `'
            if attributes['minimum']
              comparator = attributes['exclusiveMinimum'] ? '<' : '<='
              range += "#{attributes['minimum'].to_json} #{comparator} "
            end
            range += ' X '
            if attributes['maximum']
              comparator = attributes['exclusiveMaximum'] ? '<' : '<='
              range += " #{comparator} #{attributes['maximum'].to_json}"
            end
            range += '`'
            result.push(converter.convert(range.to_s))
        end

        if attributes['enum']
            # result.push(converter.convert('Допустимые значения: ' + [*attributes['enum']].map { |e| "`#{e.to_json}`" }.join(', ')))
            result.push(converter.convert('**Допустимые значения:** ' + [*attributes['enum']].map { |e| "`#{e}`" }.join(', ')))
        end

        if attributes['pattern']
            result.push(converter.convert("**Формат:** `#{attributes['pattern']}`"))
        end

        if attributes['minLength'] || attributes['maxLength']
            description = '**Длина:** `'
            if attributes['minLength']
              description += "#{attributes['minLength'].to_json}"
            end
            unless attributes['minLength'] == attributes['maxLength']
              if attributes['maxLength']
                unless attributes['minLength']
                  description += '0'
                end
                description += "..#{attributes['maxLength'].to_json}"
              else
                description += '..∞'
              end
            end
            description += '`'
            result.push(converter.convert(description.to_s))
        end

        if attributes.has_key?('example')
            example =  '**Пример:** ' + if attributes['example'].is_a?(Hash) && attributes['example'].has_key?('oneOf')
                            attributes['example']['oneOf'].map { |e| "`#{e.to_json}`" }.join(' or ')
                        else
                            "`#{attributes['example'].to_json}`"
                        end
            result.push(converter.convert(example.to_s))
        end

        if parent.has_key?('required') && parent['required'].include?(name)
            result.push(converter.convert('**Обязательный параметр.**'))
        else
            #
            # result.push(converter.convert('**Необязательный параметр.**'))
        end
        result
    end

    # params:
    # 1 - parameter name to render (string)
    # 2 - parameter attributes (hash)
    # 3 - parent item data (hash)
    def format_schema(name, attributes, parent)
        result = Array.new()
        result.push('<li>')

        if attributes.has_key?('type')
            if attributes.has_key?("items")
                result.push(format_key_name(name)+ '(<i>' +  format_type(attributes["type"], attributes["items"]["type"]) + '</i>)')
            else
                result.push(format_key_name(name)+ '(<i>' +  format_type(attributes["type"], nil) + '</i>)')
            end
        else
            result.push(format_key_name(name))
        end

        result.push(format_attribute(name, attributes, parent))

        if attributes.has_key?("properties")
            result.push('<ul>')
            attributes["properties"].each do |key, value|
                result.push(format_schema(key, value, attributes ))
            end
            result.push('</ul>')
        elsif attributes.has_key?('items')
            if attributes['items'].has_key?("properties")
                result.push('<ul>')
                attributes['items']["properties"].each do |item_key, item_value|
                    result.push(format_schema(item_key, item_value, attributes['items'] ))
                end
                result.push('</ul>')
            end
        else
        #           result.push("no properties for #{name}")
        end
        result.push('</li>')
        result.join
    end

    def format_crd(input)
        result = []
        if ( input.has_key?("spec") and input["spec"].has_key?("validation") and
           input["spec"]["validation"].has_key?("openAPIV3Schema")  ) or (input.has_key?("spec") and input["spec"].has_key?("versions"))
           then
            converter = Jekyll::Converters::Markdown::KramdownParser.new(Jekyll.configuration())

            if input["spec"].has_key?("validation") and input["spec"]["validation"].has_key?("openAPIV3Schema") then
                # v1beta1 CRD

                result.push(converter.convert("## " + input["spec"]["names"]["kind"]))
                result.push('<p><font size="-1">Scope: ' + input["spec"]["scope"])
                if input["spec"].has_key?("version") then
                   result.push('<br/>Version: ' + input["spec"]["version"] + '</font></p>')
                end

                if input["spec"]["validation"]["openAPIV3Schema"].has_key?("description")
                   result.push(converter.convert(input["spec"]["validation"]["openAPIV3Schema"]["description"]))
                end

                if input["spec"]["validation"]["openAPIV3Schema"].has_key?('properties')
                    result.push('<ul>')
                    input["spec"]["validation"]["openAPIV3Schema"]['properties'].each do |key, value|
                    result.push(format_schema(key, value, input["spec"]["validation"]["openAPIV3Schema"] ))
                    end
                    result.push('</ul>')
                end
            elsif input.has_key?("spec") and input["spec"].has_key?("versions") then
                # v1+ CRD

                 result.push(converter.convert("## " + input["spec"]["names"]["kind"]))
                 input["spec"]["versions"].each do |item|
                    if input["spec"]["versions"].length == 1 then
                        result.push('<p><font size="-1">Scope: ' + input["spec"]["scope"])
                        result.push('<br/>Version: ' + item['name'] + '</font></p>')
                    else
                        result.push(converter.convert("### " + item['name'] + ' {#' + input["spec"]["names"]["kind"].downcase + '-' + item['name'].downcase + '}'))
                        result.push('<p><font size="-1">Scope: ' + input["spec"]["scope"] + '</font></p>')
                    end

                    if item.has_key?('schema') and item['schema'].has_key?('openAPIV3Schema') and
                       item['schema']['openAPIV3Schema'].has_key?("description")
                       result.push(converter.convert(item['schema']['openAPIV3Schema']['description']))
                    end

                    if item['schema']['openAPIV3Schema'].has_key?('properties')
                        header = '<ul>'
                        item['schema']['openAPIV3Schema']['properties'].each do |key, value|
                        # skip status object
                        next if key == 'status'
                        if header != '' then
                            result.push(header)
                            header = ''
                        end
                        result.push(format_schema(key, value, item['schema']['openAPIV3Schema'] ))
                        end
                        if header == '' then
                            result.push('</ul>')
                        end
                    end
                 end
            end
        end
        result.join
    end
  end
end

Liquid::Template.register_filter(Jekyll::Openapi)
