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

        result.push("<br/> **default:** `#{value['default'].to_json}`") if attributes['default']

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
        converter = Jekyll::Converters::Markdown::KramdownParser.new(Jekyll.configuration())

        result.push(converter.convert("## " + input["spec"]["names"]["kind"]))

        result.push('<p><font size="-1">Scope: ' + input["spec"]["scope"])
        if input["spec"].has_key?("version") then
           result.push('<br/>Version: ' + input["spec"]["version"] + '')
        end
        result.push('</font></p>')
        if input["spec"]["validation"]["openAPIV3Schema"]["description"] then
           result.push(input["spec"]["validation"]["openAPIV3Schema"]["description"])
        end

        if input["spec"]["validation"]["openAPIV3Schema"]['properties']
            result.push('<ul>')
            input["spec"]["validation"]["openAPIV3Schema"]['properties'].each do |key, value|
            result.push(format_schema(key, value, input["spec"]["validation"]["openAPIV3Schema"] ))
            end
            result.push('</ul>')
        end
        result.join
    end
  end
end

Liquid::Template.register_filter(Jekyll::Openapi)
