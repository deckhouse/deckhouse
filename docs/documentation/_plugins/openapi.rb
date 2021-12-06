module Jekyll
  module Openapi

    #
    # Return localised description
    # the source parameter is for object without i18n structure and for legacy support
    def get_i18n_description(primaryLanguage, fallbackLanguage, source=nil)
        if primaryLanguage and primaryLanguage.has_key?("description") then
            result = primaryLanguage["description"]
        elsif fallbackLanguage and fallbackLanguage.has_key?("description") then
            result = fallbackLanguage["description"]
        elsif source and source.has_key?("description") then
            result = source["description"]
        else
            result = nil
        end
        result
    end

    def convertAPIVersionChannelToInt(channel)
      return 0 if channel == 'alpha'
      return 1 if channel == 'beta'
      2
    end

    # 1 if a more stable than b or has higher version
    # -1 if a less stable than b or has lower version
    def compareAPIVersion(a,b)
        version = a['name'].scan(/v([1-9]+[0-9]*)((alpha|beta)([1-9]+[0-9]*))?/).flatten
        aVersion = {"majVersion" => version[0], "stability" => convertAPIVersionChannelToInt(version[2]), "minVersion" => version[3] ? version[3] : 3 }
        version = b['name'].scan(/v([1-9]+[0-9]*)((alpha|beta)([1-9]+[0-9]*))?/).flatten
        bVersion = {"majVersion" => version[0], "stability" => convertAPIVersionChannelToInt(version[2]), "minVersion" => version[3] ? version[3] : 3 }
        return -( aVersion["majVersion"] <=> bVersion["majVersion"] ) if aVersion["majVersion"] != bVersion["majVersion"]
        return -( aVersion["stability"] <=> bVersion["stability"] ) if aVersion["stability"] != bVersion["stability"]
        -( aVersion["minVersion"] <=> bVersion["minVersion"] )
    end

    def get_i18n_term(term)
        lang = @context.registers[:page]["lang"]
        i18n = @context.registers[:site].data["i18n"]["common"]

        if ! i18n[term]
            result = term
            puts "NOTE: No i18n for the term '" + term + "'!"
        else
            result =i18n[term][lang]
        end
        result
    end

    def get_hash_value(input, *keys)
        input ? input.dig(*keys) : nil
    end

    def format_type(first_type, second_type)
        lang = @context.registers[:page]["lang"]
        i18n = @context.registers[:site].data["i18n"]["common"]

        if !i18n[first_type] then
            result = first_type
            puts "NOTE: No i18n for the '" + first_type + "' type!"
        else
            result = i18n[first_type][lang]
        end
        if second_type then
            result += ' ' + i18n['of'][lang]
            if !i18n[second_type] then
                result += " #{second_type}"
                puts "NOTE: No i18n for the 'of " + first_type + "' type!"
            else
                result += ' ' + i18n['of_' + second_type][lang]
            end
        end
        result
    end

    def format_attribute(name, attributes, parent, primaryLanguage = nil, fallbackLanguage = nil)
        result = Array.new()
        exampleObject = nil
        converter = Jekyll::Converters::Markdown::KramdownParser.new(Jekyll.configuration())

        result.push(sprintf(%q(<div class="resources__prop_description">%s</div>),converter.convert(get_i18n_description(primaryLanguage, fallbackLanguage, attributes)))) if attributes['description']

        if attributes.has_key?('x-doc-default')
            if attributes['x-doc-default'].is_a?(Array)
                result.push(sprintf(%q(<p><span class="resources__attr_name">%s:</span> <span class="resources__attr_content"><code>%s</code></span></p>), get_i18n_term("default_value").capitalize, attributes['x-doc-default'].to_json))
            else
                if attributes['type'] == 'string'
                    result.push(sprintf(%q(<p><span class="resources__attr_name">%s:</span> <span class="resources__attr_content"><code>"%s"</code></span></p>), get_i18n_term("default_value").capitalize, attributes['x-doc-default']))
                else
                    result.push(sprintf(%q(<p><span class="resources__attr_name">%s:</span> <span class="resources__attr_content"><code>%s</code></span></p>), get_i18n_term("default_value").capitalize, attributes['x-doc-default']))
                end
            end
        elsif attributes.has_key?('default')
            if attributes['default'].is_a?(Array)
                result.push(sprintf(%q(<p><span class="resources__attr_name">%s:</span> <span class="resources__attr_content"><code>%s</code></span></p>), get_i18n_term("default_value").capitalize, attributes['default'].to_json))
            else
                if attributes['type'] == 'string'
                    result.push(sprintf(%q(<p><span class="resources__attr_name">%s:</span> <span class="resources__attr_content"><code>"%s"</code></span></p>), get_i18n_term("default_value").capitalize, attributes['default']))
                else
                    result.push(sprintf(%q(<p><span class="resources__attr_name">%s:</span> <span class="resources__attr_content"><code>%s</code></span></p>), get_i18n_term("default_value").capitalize, attributes['default']))
                end
            end
        end

        if attributes.has_key?('x-doc-versionType')
          case attributes['x-doc-versionType']
          when "ee"
            result.push(converter.convert('**' + @context.registers[:site].data['i18n']['features']['ee']['ru'].capitalize + '**'))
          when "experimental"
            result.push(converter.convert('**' + @context.registers[:site].data['i18n']['features']['experimental'][lang].capitalize + '**'))
          end
        end

        if attributes['minimum'] || attributes['maximum']
            valuesRange = '<p><span class="resources__attr_name">' + get_i18n_term("allowed_values").capitalize + ':</span> '
            valuesRange += '<span class="resources__attr_content"><code>'
            if attributes['minimum']
              comparator = attributes['exclusiveMinimum'] ? '<' : '<='
              valuesRange += "#{attributes['minimum'].to_json} #{comparator} "
            end
            valuesRange += ' X '
            if attributes['maximum']
              comparator = attributes['exclusiveMaximum'] ? '<' : '<='
              valuesRange += " #{comparator} #{attributes['maximum'].to_json}"
            end
            valuesRange += "</code></span></p>"
            result.push(converter.convert(valuesRange.to_s))
        end

        if attributes['enum']
            enum_result = '<p><span class="resources__attr_name">' + get_i18n_term("allowed_values").capitalize
            if name == "" and parent['type'] == 'array'
                enum_result += ' ' + get_i18n_term("allowed_values_of_array")
            end
            result.push(enum_result + ':</span> <span class="resources__attr_content">'+ [*attributes['enum']].map { |e| "<code>#{e}</code>" }.join(', ') + '</span></p>')
        end

        if attributes['pattern']
            result.push(sprintf(%q(<p><span class="resources__attr_name">%s:</span> <code class="resources__attr_content">%s</code></p>),get_i18n_term("pattern").capitalize, attributes['pattern']))
        end

        if attributes['minLength'] || attributes['maxLength']
            description = %Q(<p><span class="resources__attr_name">#{get_i18n_term('length').capitalize}</span>: <span class="resources__attr_content"><code>)
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
            description += '</code></span></p>'
            result.push(converter.convert(description.to_s))
        end

        if attributes.has_key?('x-doc-example')
            exampleObject = attributes['x-doc-example']
        elsif attributes.has_key?('example')
            exampleObject = attributes['example']
        elsif attributes.has_key?('x-examples')
            exampleObject = attributes['x-examples']
        end
        if exampleObject != nil
            example =  %Q(<p><span class="resources__attr_name">#{get_i18n_term('example').capitalize}:</span> <span class="resources__attr_content">) +
                        if exampleObject.is_a?(Hash) && exampleObject.has_key?('oneOf')
                            exampleObject['oneOf'].map { |e| "<code>#{e.to_json}</code>" }.join(' ' + get_i18n_term('or') + ' ')
                        elsif exampleObject.is_a?(Array) || exampleObject.is_a?(Hash)
                            exampleObject.map { |e| "<code>#{e.to_json}</code>" }.join(', ')
                        else
                            if exampleObject =~ /\`\`\`|\n/
                                "\n\n#{exampleObject}"
                            else
                              if attributes['type'] == 'string'
                                "<code>\"#{exampleObject}\"</code>"
                              else
                                "<code>#{exampleObject}</code>"
                              end
                            end
                        end
            example += '</span></p>'
            result.push(converter.convert(example.to_s))
        end

        if parent.has_key?('required') && parent['required'].include?(name)
            result.push(%Q(<p><span class="resources__attr_name required">#{get_i18n_term('required_value_sentence')}</span></p>))
        elsif attributes.has_key?('x-doc-required')
            if attributes['x-doc-required']
                result.push(%Q(<p><span class="resources__attr_name required">#{get_i18n_term('required_value_sentence')}</span></p>))
            else
                result.push(%Q(<p><span class="resources__attr_name not_required">#{get_i18n_term('not_required_value_sentence')}</span></p>))
            end
        else
            # Not sure if there will always be an optional value here...
            # result.push(converter.convert('**' + get_i18n_term('not_required_value_sentence')  + '**'))
        end
        result
    end

    # params:
    # 1 - parameter name to render (string)
    # 2 - parameter attributes (hash)
    # 3 - parent item data (hash)
    # 4 - object with primary language data
    # 5 - object with language data which use if there is no data in primary language
    def format_schema(name, attributes, parent, primaryLanguage = nil, fallbackLanguage = nil, parentID = "")
        result = Array.new()

        linkAnchor = name.downcase
        if parentID.length > 0
           linkAnchor = sprintf(%q(%s-%s), parentID, linkAnchor)
        end

        if name != ""
            name_text = ''
            result.push('<li>')
            attributes_type = ''
            if attributes.is_a?(Hash)
              if attributes.has_key?('type')
                 attributes_type = attributes["type"]
              elsif attributes.has_key?('x-kubernetes-int-or-string')
                 attributes_type = "x-kubernetes-int-or-string"
              end
            end

            if attributes['x-doc-deprecated']
                name_text = sprintf(%q(<span id="%s" data-anchor-id="%s" class="resources__prop_name anchored deprecated">%s</span>), linkAnchor, linkAnchor, name)
            else
                name_text = sprintf(%q(<span id="%s" data-anchor-id="%s" class="resources__prop_name anchored">%s</span>), linkAnchor, linkAnchor, name)
            end

            if attributes_type != ''
                if attributes.is_a?(Hash) and attributes.has_key?("items")
                    name_text += sprintf(%q(<span class="resources__prop_type">%s</span>), format_type(attributes_type, attributes["items"]["type"]))
                else
                    name_text += sprintf(%q(<span class="resources__prop_type">%s</span>), format_type(attributes_type, nil))
                end
            end

            result.push(name_text)
        end

#         result.push(format_attribute(name, attributes, parent, primaryLanguage, fallbackLanguage)) if attributes.is_a?(Hash)
        result.push(format_attribute(name, attributes, parent, primaryLanguage, fallbackLanguage)) if attributes.is_a?(Hash)

        if attributes.is_a?(Hash) and attributes.has_key?("properties")
            result.push('<ul>')
            attributes["properties"].sort.to_h.each do |key, value|
                result.push(format_schema(key, value, attributes, get_hash_value(primaryLanguage, "properties", key), get_hash_value(fallbackLanguage, "properties", key), linkAnchor))
            end
            result.push('</ul>')
        elsif attributes.is_a?(Hash) and  attributes.has_key?('items')
            if get_hash_value(attributes,'items','properties')
                # object items
                result.push('<ul>')
                attributes['items']["properties"].sort.to_h.each do |item_key, item_value|
                    result.push(format_schema(item_key, item_value, attributes['items'], get_hash_value(primaryLanguage,"items", "properties", item_key) , get_hash_value(fallbackLanguage,"items", "properties", item_key), linkAnchor))
                end
                result.push('</ul>')
            else
                result.push(format_schema("", attributes['items'], attributes, get_hash_value(primaryLanguage,'items'), get_hash_value(fallbackLanguage,'items'), linkAnchor ))
            end
        else
            # result.push("no properties for #{name}")
        end

        if name != ""
            result.push('</li>')
        end
        result.join
    end

    def format_crd(input)

        return nil if !input

        if ( @context.registers[:page]["lang"] == 'en' )
            fallbackLanguageName = 'ru'
        else
            fallbackLanguageName = 'en'
        end
        result = []
        if !( input.has_key?('i18n'))
           input['i18n'] = {}
        end
        if !( input['i18n'].has_key?('en'))
           input['i18n']['en'] = { "spec" => input["spec"] }
        end
        result.push('<div markdown="0">')
        if ( get_hash_value(input,'spec','validation','openAPIV3Schema')  ) or (get_hash_value(input,'spec','versions'))
           then
            converter = Jekyll::Converters::Markdown::KramdownParser.new(Jekyll.configuration())

            if get_hash_value(input,'spec','validation','openAPIV3Schema') then
                # v1beta1 CRD
                linkAnchor=sprintf(%q(v1beta1-%s), input["spec"]["names"]["kind"].downcase)
                result.push(converter.convert("## " + input["spec"]["names"]["kind"]))
                result.push('<p><font size="-1">Scope: ' + input["spec"]["scope"])
                if input["spec"].has_key?("version") then
                   result.push('<br/>Version: ' + input["spec"]["version"] + '</font></p>')
                end

                if get_hash_value(input,'spec','validation','openAPIV3Schema','description')
                   if get_hash_value(input['i18n'][@context.registers[:page]["lang"]],"spec","validation","openAPIV3Schema","description") then
                       result.push(converter.convert(get_hash_value(input['i18n'][@context.registers[:page]["lang"]],"spec","validation","openAPIV3Schema","description")))
                   elsif get_hash_value(input['i18n'][fallbackLanguageName],"spec","validation","openAPIV3Schema","description") then
                       result.push(converter.convert(input['i18n'][fallbackLanguageName]["spec"]["validation"]["openAPIV3Schema"]["description"]))
                   else
                       result.push(converter.convert(input["spec"]["validation"]["openAPIV3Schema"]["description"]))
                   end
                end

                if input["spec"]["validation"]["openAPIV3Schema"].has_key?('properties')
                    result.push('<ul class="resources">')
                    input["spec"]["validation"]["openAPIV3Schema"]['properties'].sort.to_h.each do |key, value|
                    _primaryLanguage = nil
                    _fallbackLanguage = nil

                    if  input['i18n'][@context.registers[:page]["lang"]] then
                        _primaryLanguage = get_hash_value(input['i18n'][@context.registers[:page]["lang"]],"spec","validation","openAPIV3Schema","properties",key)
                    end
                    if   input['i18n'][fallbackLanguageName] then
                        _fallbackLanguage = get_hash_value(input['i18n'][fallbackLanguageName],"spec","validation","openAPIV3Schema","properties",key)
                    end
                        result.push(format_schema(key, value, input["spec"]["validation"]["openAPIV3Schema"], _primaryLanguage, _fallbackLanguage, linkAnchor ))
                    end
                    result.push('</ul>')
                end
            elsif input.has_key?("spec") and input["spec"].has_key?("versions") then
                # v1+ CRD

                 result.push(converter.convert("## " + input["spec"]["names"]["kind"]))

                 if  input["spec"]["versions"].length > 1 then
                     result.push('<p><font size="-1">Scope: ' + input["spec"]["scope"] + '</font></p>')
                     result.push('<div class="tabs">')
                     activeStatus=" active"
                     input["spec"]["versions"].sort{ |a, b| compareAPIVersion(a,b) }.each do |item|
                         #result.push(" onclick=\"openTab(event, 'tabs__btn', 'tabs__content', " + input["spec"]["names"]["kind"].downcase + '_' + item['name'].downcase + ')">' + item['name'].downcase + '</a>')
                         result.push("<a href='javascript:void(0)' class='tabs__btn tabs__btn__%s%s' onclick=\"openTab(event, 'tabs__btn__%s', 'tabs__content__%s', '%s_%s')\">%s</a>" %
                           [ input["spec"]["names"]["kind"].downcase, activeStatus,
                             input["spec"]["names"]["kind"].downcase,
                             input["spec"]["names"]["kind"].downcase,
                             input["spec"]["names"]["kind"].downcase, item['name'].downcase,
                             item['name'].downcase ])
                         activeStatus = ""
                     end
                     result.push('</div>')
                 end

                 activeStatus=" active"
                 input["spec"]["versions"].sort{ |a, b| compareAPIVersion(a,b) }.each do |item|
                    _primaryLanguage = nil
                    _fallbackLanguage = nil

                    if input["spec"]["versions"].length == 1 then
                        result.push('<p><font size="-1">Scope: ' + input["spec"]["scope"])
                        result.push('<br/>Version: ' + item['name'] + '</font></p>')
                    else
                        #result.push(converter.convert("### " + item['name'] + ' {#' + input["spec"]["names"]["kind"].downcase + '-' + item['name'].downcase + '}'))
                        #result.push('<p><font size="-1">Scope: ' + input["spec"]["scope"] + '</font></p>')
                    end

                    if input["spec"]["versions"].length > 1 then
                        result.push("<div id='%s_%s' class='tabs__content tabs__content__%s%s'>" %
                            [ input["spec"]["names"]["kind"].downcase, item['name'].downcase,
                            input["spec"]["names"]["kind"].downcase, activeStatus ])
                        activeStatus = ""
                    end

                    if get_hash_value(item,'schema','openAPIV3Schema','description') then
                       if  input['i18n'][@context.registers[:page]["lang"]] and
                           get_hash_value(input['i18n'][@context.registers[:page]["lang"]],"spec","versions") and
                           input['i18n'][@context.registers[:page]["lang"]]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0] then
                       result.push(converter.convert(input['i18n'][@context.registers[:page]["lang"]]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0]["schema"]["openAPIV3Schema"]["description"]))
                       elsif input['i18n'][fallbackLanguageName] and
                             get_hash_value(input['i18n'][fallbackLanguageName],"spec","versions") and
                            input['i18n'][fallbackLanguageName]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0] then
                       result.push(converter.convert(input['i18n'][fallbackLanguageName]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0]["schema"]["openAPIV3Schema"]["description"]))
                       else
                           result.push('<div class="resources__prop_description">' + converter.convert(item["schema"]["openAPIV3Schema"]["description"]) + '</div>')
                       end
                    end

                    if get_hash_value(item,'schema','openAPIV3Schema','properties')
                        header = '<ul class="resources">'
                        item['schema']['openAPIV3Schema']['properties'].each do |key, value|
                        _primaryLanguage = nil
                        _fallbackLanguage = nil
                        # skip status object
                        next if key == 'status'
                        if header != '' then
                            result.push(header)
                            header = ''
                        end

                        if  input['i18n'][@context.registers[:page]["lang"]] and
                            get_hash_value(input['i18n'][@context.registers[:page]["lang"]],"spec","versions") and
                            input['i18n'][@context.registers[:page]["lang"]]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0]
                        then
                            _primaryLanguage = input['i18n'][@context.registers[:page]["lang"]]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0]
                            _primaryLanguage = get_hash_value(_primaryLanguage,'schema','openAPIV3Schema','properties',key)
                        end
                        if  input['i18n'][fallbackLanguageName] and
                            get_hash_value(input['i18n'][fallbackLanguageName],"spec","versions") and
                            input['i18n'][fallbackLanguageName]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0]
                        then
                            _fallbackLanguage = input['i18n'][fallbackLanguageName]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0]
                            _fallbackLanguage = get_hash_value(_fallbackLanguage,'schema','openAPIV3Schema','properties',key)
                        end

                        linkAnchor=sprintf(%q(%s-%s), input["spec"]["names"]["kind"].downcase, item['name'].downcase)

                        result.push(format_schema(key, value, item['schema']['openAPIV3Schema'] , _primaryLanguage, _fallbackLanguage, linkAnchor))
                        end
                        if header == '' then
                            result.push('</ul>')
                        end
                    end

                    if get_hash_value(input,'spec','versions').length > 1 then
                        result.push("</div>")
                    end

                 end
            end
        end
        result.push('</div>')
        result.join
    end

    #
    # Returns configuration module content from the openAPI spec
    def format_configuration(input)
        result = []
        result.push('<div markdown="0">')
        if !( input.has_key?('i18n'))
           input['i18n'] = {}
        end
        if !( input['i18n'].has_key?('en'))
           input['i18n']['en'] = { "properties" => input['properties'] }
        end
        if ( input.has_key?("properties"))
           then
            converter = Jekyll::Converters::Markdown::KramdownParser.new(Jekyll.configuration())

            result.push('<ul class="resources">')
            input['properties'].sort.to_h.each do |key, value|
                _primaryLanguage = nil
                _fallbackLanguage = nil

                if ( input['i18n'].has_key?(@context.registers[:page]["lang"]) and input['i18n'][@context.registers[:page]["lang"]].has_key?("properties") )
                    _primaryLanguage = input['i18n'][@context.registers[:page]["lang"]]["properties"][key]
                end
                if ( @context.registers[:page]["lang"] == 'en' )
                    fallbackLanguageName = 'ru'
                else
                    fallbackLanguageName = 'en'
                end
                if ( input['i18n'].has_key?(fallbackLanguageName) and input['i18n'][fallbackLanguageName].has_key?("properties") )
                    _fallbackLanguage = input['i18n'][fallbackLanguageName]["properties"][key]
                end

                result.push(format_schema(key, value, input, _primaryLanguage, _fallbackLanguage, "parameters" ))
            end
            result.push('</ul>')
        end
        result.push('</div>')
        result.join
    end

    def format_cluster_configuration(input)
        converter = Jekyll::Converters::Markdown::KramdownParser.new(Jekyll.configuration())
        result = []
        result.push('<div markdown="0">')
        result.push(converter.convert('## '+ input["kind"]))

        for i in 0..(input["apiVersions"].length-1)
          result.push("<p><font size='-1'>Version: " + input["apiVersions"][i]["apiVersion"] + "</font></p>")
          item=input["apiVersions"][i]["openAPISpec"]
          item["i18n"]={}
          item["i18n"]["ru"]=get_hash_value(input,"i18n","ru","apiVersions",i,"openAPISpec")
          result.push(format_configuration(item))
        end
        result.push('</div>')
        result.join
    end
  end
end

Liquid::Template.register_filter(Jekyll::Openapi)
