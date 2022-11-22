CONVERTER = Jekyll::Converters::Markdown::KramdownParser.new(Jekyll.configuration())

module Jekyll
  module Openapi
    #
    # Return localised description
    # the source parameter is for object without i18n structure and for legacy support
    def get_i18n_description(primaryLanguage, fallbackLanguage, source=nil)
        if get_hash_value(primaryLanguage, "description") then
            result = primaryLanguage["description"]
        elsif get_hash_value(fallbackLanguage, "description") then
            result = fallbackLanguage["description"]
        elsif get_hash_value(source, "description") then
            result = source["description"]
        else
            result = ''
        end

        if get_hash_value(primaryLanguage, "items", "properties") or get_hash_value(fallbackLanguage, "items", "properties")
            if get_hash_value(primaryLanguage, "items", "description") then
              result += %Q(\n\n#{primaryLanguage["items"]["description"]})
            elsif get_hash_value(fallbackLanguage, "items", "description") then
              result += %Q(\n\n#{fallbackLanguage["items"]["description"]})
            end
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

    def format_examples(name, attributes)
        result = Array.new()
        exampleObject = nil

        begin
          if attributes.has_key?('x-doc-example')
              exampleKeyToUse = 'x-doc-example'
          elsif attributes.has_key?('example')
              exampleKeyToUse = 'example'
          elsif attributes.has_key?('x-examples')
              exampleKeyToUse = 'x-examples'
          end
        rescue
          puts attributes
        end

        exampleObject = attributes[exampleKeyToUse]

        if exampleObject != nil
            exampleObjectIsArrayOfExamples =  false
            if exampleKeyToUse == 'x-examples' then
                exampleObjectIsArrayOfExamples =  true
            end
            if exampleKeyToUse == 'x-doc-example' then
                if attributes['type'] == 'array' and exampleObject.is_a?(Array) then
                   exampleObjectIsArrayOfExamples =  true
                end
            end
            if attributes['type'] == 'array' and !exampleObject.is_a?(Array) then
               if exampleKeyToUse == 'example' then
                   exampleObject = [exampleObject]
                   exampleObjectIsArrayOfExamples =  true
               end
            end

            if exampleObjectIsArrayOfExamples and exampleObject.length > 1 then
                exampleTitle = get_i18n_term('examples').capitalize
            else
                exampleTitle = get_i18n_term('example').capitalize
            end

            example =  %Q(<p class="resources__attrs"><span class="resources__attrs_name">#{exampleTitle}:</span>)
            exampleContent = ""
                        if exampleObject.is_a?(Hash) && exampleObject.has_key?('oneOf')
                            exampleContent = %Q(```yaml\n#{(if name then {name => exampleObject['oneOf']} else exampleObject['oneOf'] end).to_yaml.sub(/^---(\n| ){1}/,'')}```)
                        elsif exampleObject.is_a?(Hash)
                            exampleContent = %Q(```yaml\n#{(if name then {name => exampleObject} else exampleObject end).to_yaml.sub(/^---(\n| ){1}/,'')}```)
                        elsif exampleObjectIsArrayOfExamples and (exampleObject.length == 1)
                            if exampleObject[0].class.to_s == "String" and exampleObject[0] =~ /\`\`\`|\n/
                                exampleContent = "#{exampleObject[0]}"
                            else
                                exampleContent = %Q(```yaml\n#{(if name then {name => exampleObject[0]} else exampleObject[0] end).to_yaml.sub(/^---(\n| ){1}/,'')}```)
                            end
                        elsif exampleObjectIsArrayOfExamples and (exampleObject.length > 1)
                            exampleObject.each do | value |
                                if value == nil then continue end
                                if exampleContent.length > 0 then exampleContent = exampleContent + "\n" end
                                exampleContent = %Q(#{exampleContent}\n```yaml\n#{(if name then {name => value} else value end).to_yaml.sub(/^---(\n| ){1}/,'')}```)
                            end
                        elsif exampleObject.is_a?(Array)
                            exampleContent = %Q(```yaml\n#{( if name then {name => exampleObject} else exampleObject end).to_yaml.sub(/^---(\n| ){1}/,'')}```)
                        else
                            if exampleObject =~ /\`\`\`|\n/
                                exampleContent = "#{exampleObject}"
                            elsif attributes['type'] == 'boolean' then
                                exampleContent = %Q(```yaml\n#{(if name then {name => (exampleObject and true)} else (exampleObject and true) end).to_yaml.sub(/^---(\n| ){1}/,'')}```)
                            elsif attributes['type'] == 'integer' or attributes['type'] == 'number' then
                                exampleContent = %Q(```yaml\n#{(if name then {name => exampleObject.to_i} else exampleObject.to_i end).to_yaml.sub(/^---(\n| ){1}/,'')}```)
                            else
                                exampleContent = %Q(```yaml\n#{(if name then {name => exampleObject.to_s} else exampleObject.to_s end).to_yaml.sub(/^---(\n| ){1}/,'')}```)
                            end

                        end
            exampleContent = CONVERTER.convert(exampleContent).delete_prefix('<p>').sub(/<\/p>[\s]*$/,"")
            if exampleContent.match?(/^<div/)
                result.push(%Q(#{example}</p>#{exampleContent}))
            else
                result.push(%Q(#{example} #{exampleContent}</p>))
            end
        end
        result
    end

    def format_attribute(name, attributes, parent, primaryLanguage = nil, fallbackLanguage = nil)
        result = Array.new()
        exampleObject = nil
        lang = @context.registers[:page]["lang"]

        if parent.has_key?('required') && parent['required'].include?(name)
            result.push(%Q(<p class="resources__attrs required"><span class="resources__attrs_name required">#{get_i18n_term('required_value_sentence')}</span></p>))
        elsif attributes.has_key?('x-doc-required')
            if attributes['x-doc-required']
                result.push(%Q(<p class="resources__attrs required"><span class="resources__attrs_name required">#{get_i18n_term('required_value_sentence')}</span></p>))
            else
                result.push(%Q(<p class="resources__attrs required"><span class="resources__attrs_name not_required">#{get_i18n_term('not_required_value_sentence')}</span></p>))
            end
        else
            # Not sure if there will always be an optional value here...
            # result.push(CONVERTER.convert('**' + get_i18n_term('not_required_value_sentence')  + '**'))
        end

        result.push(sprintf(%q(<div class="resources__prop_description">%s</div>),CONVERTER.convert(get_i18n_description(primaryLanguage, fallbackLanguage, attributes)))) if attributes['description']

        if attributes.has_key?('x-doc-default')
            if attributes['x-doc-default'].is_a?(Array) or attributes['x-doc-default'].is_a?(Hash)
                if !( attributes['x-doc-default'].is_a?(Hash) and attributes['x-doc-default'].length < 1 )
                    result.push(sprintf(%q(<p class="resources__attrs"><span class="resources__attrs_name">%s:</span> <span class="resources__attrs_content"><code>%s</code></span></p>), get_i18n_term("default_value").capitalize, attributes['x-doc-default'].to_json))
                end
            else
                if attributes['type'] == 'string'
                    result.push(sprintf(%q(<p class="resources__attrs"><span class="resources__attrs_name">%s:</span> <span class="resources__attrs_content"><code>"%s"</code></span></p>), get_i18n_term("default_value").capitalize, attributes['x-doc-default']))
                else
                    result.push(sprintf(%q(<p class="resources__attrs"><span class="resources__attrs_name">%s:</span> <span class="resources__attrs_content"><code>%s</code></span></p>), get_i18n_term("default_value").capitalize, attributes['x-doc-default']))
                end
            end
        elsif attributes.has_key?('default')
            if attributes['default'].is_a?(Array) or attributes['default'].is_a?(Hash)
                if !( attributes['default'].is_a?(Hash) and attributes['default'].length < 1 )
                    result.push(sprintf(%q(<p class="resources__attrs"><span class="resources__attrs_name">%s:</span> <span class="resources__attrs_content"><code>%s</code></span></p>), get_i18n_term("default_value").capitalize, attributes['default'].to_json))
                end
            else
                if attributes['type'] == 'string'
                    result.push(sprintf(%q(<p class="resources__attrs"><span class="resources__attrs_name">%s:</span> <span class="resources__attrs_content"><code>"%s"</code></span></p>), get_i18n_term("default_value").capitalize, attributes['default']))
                else
                    result.push(sprintf(%q(<p class="resources__attrs"><span class="resources__attrs_name">%s:</span> <span class="resources__attrs_content"><code>%s</code></span></p>), get_i18n_term("default_value").capitalize, attributes['default']))
                end
            end
        end

        if attributes.has_key?('x-doc-d8Revision')
          case attributes['x-doc-d8Revision']
          when "ee"
            result.push(CONVERTER.convert('**' + @context.registers[:site].data['i18n']['features']['ee'][lang].capitalize + '**'))
          end
        end

        if attributes.has_key?('x-doc-featureStatus')
          case attributes['x-doc-featureStatus']
          when "proprietaryOkmeter"
            result.push(CONVERTER.convert('**' + @context.registers[:site].data['i18n']['features']['proprietaryOkmeter'][lang].capitalize + '**'))
          when "experimental"
            result.push(CONVERTER.convert('**' + @context.registers[:site].data['i18n']['features']['experimental'][lang].capitalize + '**'))
          end
        end

        if attributes.has_key?('minimum') or attributes.has_key?('maximum')
            valuesRange = '<p class="resources__attrs"><span class="resources__attrs_name">' + get_i18n_term("allowed_values").capitalize + ':</span> '
            valuesRange += '<span class="resources__attrs_content"><code>'
            if attributes.has_key?('minimum')
              comparator = attributes['exclusiveMinimum'] ? '<' : '<='
              valuesRange += "#{attributes['minimum'].to_json} #{comparator} "
            end
            valuesRange += ' X '
            if attributes.has_key?('maximum')
              comparator = attributes['exclusiveMaximum'] ? '<' : '<='
              valuesRange += " #{comparator} #{attributes['maximum'].to_json}"
            end
            valuesRange += "</code></span></p>"
            result.push(CONVERTER.convert(valuesRange.to_s))
        end

        if attributes.has_key?('enum')
            enum_result = '<p class="resources__attrs"><span class="resources__attrs_name">' + get_i18n_term("allowed_values").capitalize
            if name == "" and parent['type'] == 'array'
                enum_result += ' ' + get_i18n_term("allowed_values_of_array")
            end
            result.push(enum_result + ':</span> <span class="resources__attrs_content">'+ [*attributes['enum']].map { |e| "<code>#{e}</code>" }.join(', ') + '</span></p>')
        end

        if attributes.has_key?('pattern')
            result.push(sprintf(%q(<p class="resources__attrs"><span class="resources__attrs_name">%s:</span> <code class="resources__attrs_content">%s</code></p>),get_i18n_term("pattern").capitalize, attributes['pattern']))
        end

        if attributes.has_key?('minLength') || attributes.has_key?('maxLength')
            if attributes.has_key?('minLength') && attributes.has_key?('maxLength')
                caption = 'length'
                lengthValue = "#{attributes['minLength'].to_json}..#{attributes['maxLength'].to_json}"
            elsif attributes.has_key?('minLength')
                caption = 'min_length'
                lengthValue = "#{attributes['minLength'].to_json}"
            else
                caption = 'max_length'
                lengthValue = "#{attributes['maxLength'].to_json}"
            end
            unless attributes['type'] == 'string' && caption == 'min_length'
                description = %Q(<p class="resources__attrs"><span class="resources__attrs_name">#{get_i18n_term(caption).capitalize}</span>: <span class="resources__attrs_content"><code>#{lengthValue}</code></span></p>)
                result.push(CONVERTER.convert(description.to_s))
            end
        end

        result.push(format_examples(name, attributes))

        result
    end

    # params:
    # 1 - parameter name to render (string)
    # 2 - parameter attributes (hash)
    # 3 - parent item data (hash)
    # 4 - object with primary language data
    # 5 - object with language data which use if there is no data in primary language
    def format_schema(name, attributes, parent, primaryLanguage = nil, fallbackLanguage = nil, ancestors = [], resourceName = '', versionAPI = '')
        result = Array.new()

        if name.nil?
            fullPath = ancestors + ['element']
            parameterTitle = get_i18n_term('element_of_array').capitalize
        elsif name != ''
            fullPath = ancestors + [name]
            parameterTitle = name
        else
            puts 'ERROR: Empty parameter name!'
            puts 'Parent: ', parent
            puts 'Attributes: ', attributes
            abort
        end

        linkAnchor = fullPath.join('-').downcase
        pathString = fullPath.slice(1,fullPath.length-1).join('.')

        # Data for search index
        if name and name.length > 0 and ! @context.registers[:site].data['search']['skipParameters'].include?(name)
            searchItemData = Hash.new
            # searchItemData['title'] = sprintf(%q(%s: %s), @context.registers[:page]["title"], pathString.gsub(/\./,'->') )
            searchItemData['title'] = sprintf(%q(%s -> %s), @context.registers[:page]["title"], name )
            searchItemData['url'] = sprintf(%q(%s#%s), @context.registers[:page]["url"], linkAnchor)
            if attributes['description']
                if resourceName and resourceName.length > 0
                    searchItemData['content'] = sprintf(%q(%s->%s %s), resourceName, versionAPI, CONVERTER.convert(get_i18n_description(primaryLanguage, fallbackLanguage, attributes)) )
                else
                    searchItemData['content'] = sprintf(%q(%s %s), versionAPI, CONVERTER.convert(get_i18n_description(primaryLanguage, fallbackLanguage, attributes)) )
                end
            end
            @context.registers[:site].data['search']['searchItems'][@context.registers[:page]["lang"]] << searchItemData
        end

        if parameterTitle != ''
            parameterTextContent = ''
            result.push('<li>')
            attributesType = ''
            if attributes.is_a?(Hash)
              if attributes.has_key?('type')
                 attributesType = attributes["type"]
              elsif attributes.has_key?('x-kubernetes-int-or-string')
                 attributesType = "x-kubernetes-int-or-string"
              end
            end

            if get_hash_value(attributes, 'x-doc-deprecated')
                parameterTextContent = sprintf(%q(<span id="%s" data-anchor-id="%s" class="resources__prop_title anchored"><span data-tippy-content="%s">%s</span><span data-tippy-content="%s" class="resources__prop_is_deprecated">%s</span></span>), linkAnchor, linkAnchor, pathString, parameterTitle, get_i18n_term('deprecated_parameter_hint'), get_i18n_term('deprecated_parameter') )
            else
                parameterTextContent = sprintf(%q(<span id="%s" data-anchor-id="%s" class="resources__prop_name anchored" data-tippy-content="%s">%s</span>), linkAnchor, linkAnchor, pathString, parameterTitle)
            end

            if attributesType != ''
                if attributes.is_a?(Hash) and attributes.has_key?("items")
                    parameterTextContent += sprintf(%q(<span class="resources__prop_type">%s</span>), format_type(attributesType, attributes["items"]["type"]))
                else
                    parameterTextContent += sprintf(%q(<span class="resources__prop_type">%s</span>), format_type(attributesType, nil))
                end
            end

            result.push(parameterTextContent)
        end

        result.push(format_attribute(name, attributes, parent, primaryLanguage, fallbackLanguage)) if attributes.is_a?(Hash)

        if attributes.is_a?(Hash) and attributes.has_key?("properties")
            result.push('<ul>')
            attributes["properties"].sort.to_h.each do |key, value|
                result.push(format_schema(key, value, attributes, get_hash_value(primaryLanguage, "properties", key), get_hash_value(fallbackLanguage, "properties", key), fullPath, resourceName, versionAPI))
            end
            result.push('</ul>')
        elsif attributes.is_a?(Hash) and  attributes.has_key?('items')
            if get_hash_value(attributes,'items','properties')
                #  Array of objects
                result.push('<ul>')
                attributes['items']["properties"].sort.to_h.each do |item_key, item_value|
                    result.push(format_schema(item_key, item_value, attributes['items'], get_hash_value(primaryLanguage,"items", "properties", item_key) , get_hash_value(fallbackLanguage,"items", "properties", item_key), fullPath, resourceName, versionAPI))
                end
                result.push('</ul>')
            else
                # Array of non-objects (string, integer, etc.)
                keysToShow = ['description', 'example', 'x-examples', 'x-doc-example', 'enum', 'default', 'x-doc-default', 'minimum', 'maximum', 'pattern', 'minLength', 'maxLength']
                if (attributes['items'].keys & keysToShow).length > 0
                    lang = @context.registers[:page]["lang"]
                    i18n = @context.registers[:site].data["i18n"]["common"]
                    result.push('<ul>')
                    result.push(format_schema(nil, attributes['items'], attributes, get_hash_value(primaryLanguage,"items") , get_hash_value(fallbackLanguage,"items"), fullPath, resourceName, versionAPI))
                    result.push('</ul>')
                end
            end
        else
            # result.push("no properties for #{name}")
        end

        if parameterTitle != ''
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
            if get_hash_value(input,'spec','validation','openAPIV3Schema','properties') then
                # v1beta1 CRD
                versionAPI = 'v1beta1'
                resourceName = input["spec"]["names"]["kind"]
                fullPath = [sprintf(%q(v1beta1-%s), input["spec"]["names"]["kind"])]
                result.push(CONVERTER.convert("## " + input["spec"]["names"]["kind"]))
                result.push('<p><font size="-1">Scope: ' + input["spec"]["scope"])
                if input["spec"].has_key?("version") then
                   result.push('<br/>Version: ' + input["spec"]["version"] + '</font></p>')
                end

                if get_hash_value(input,'spec','validation','openAPIV3Schema','description')
                   if get_hash_value(input['i18n'][@context.registers[:page]["lang"]],"spec","validation","openAPIV3Schema","description") then
                       result.push(CONVERTER.convert(get_hash_value(input['i18n'][@context.registers[:page]["lang"]],"spec","validation","openAPIV3Schema","description")))
                   elsif get_hash_value(input['i18n'][fallbackLanguageName],"spec","validation","openAPIV3Schema","description") then
                       result.push(CONVERTER.convert(input['i18n'][fallbackLanguageName]["spec"]["validation"]["openAPIV3Schema"]["description"]))
                   else
                       result.push(CONVERTER.convert(input["spec"]["validation"]["openAPIV3Schema"]["description"]))
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
                        result.push(format_schema(key, value, input["spec"]["validation"]["openAPIV3Schema"], _primaryLanguage, _fallbackLanguage, fullPath, resourceName, versionAPI))
                    end
                    result.push('</ul>')
                end
            elsif input.has_key?("spec") and input["spec"].has_key?("versions") then
                # v1+ CRD

                 hasNonEmptyFields = false
                 input["spec"]["versions"].each do |item|
                     if get_hash_value(item,'schema','openAPIV3Schema','properties') then
                         hasNonEmptyFields = true
                     end
                 end

                 if !hasNonEmptyFields then
                     return nil
                 end

                 result.push(CONVERTER.convert("## " + input["spec"]["names"]["kind"]))

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
                    versionAPI = item['name']

                    if input["spec"]["versions"].length == 1 then
                        result.push('<p><font size="-1">Scope: ' + input["spec"]["scope"])
                        result.push('<br/>Version: ' + item['name'] + '</font></p>')
                    else
                        #result.push(CONVERTER.convert("### " + item['name'] + ' {#' + input["spec"]["names"]["kind"].downcase + '-' + item['name'].downcase + '}'))
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
                       result.push(CONVERTER.convert(input['i18n'][@context.registers[:page]["lang"]]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0]["schema"]["openAPIV3Schema"]["description"]))
                       elsif input['i18n'][fallbackLanguageName] and
                             get_hash_value(input['i18n'][fallbackLanguageName],"spec","versions") and
                            input['i18n'][fallbackLanguageName]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0] then
                       result.push(CONVERTER.convert(input['i18n'][fallbackLanguageName]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0]["schema"]["openAPIV3Schema"]["description"]))
                       else
                           result.push('<div class="resources__prop_description">' + CONVERTER.convert(item["schema"]["openAPIV3Schema"]["description"]) + '</div>')
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

                        fullPath = [sprintf(%q(%s-%s), input["spec"]["names"]["kind"], item['name'])]
                        linkAnchor = sprintf(%q(%s-%s), input["spec"]["names"]["kind"].downcase, item['name'].downcase)
                        resourceName = input["spec"]["names"]["kind"]

                        result.push(format_schema(key, value, item['schema']['openAPIV3Schema'] , _primaryLanguage, _fallbackLanguage, fullPath, resourceName, versionAPI))
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

        if input.nil?
           input = {}
        end

        versionAPI = get_hash_value(input, 'APIversion')
        resourceName = get_hash_value(input, 'resourceName')

        if !( get_hash_value(input, 'i18n') )
           input['i18n'] = {}
        end

        if !( get_hash_value(input, 'i18n', 'en' ) )
           input['i18n']['en'] = { "properties" => input['properties'] }
        end

        configVersion = 1
        if ( get_hash_value(input, "x-config-version") ) then
          configVersion = input['x-config-version']
        end
        result.push('<p><font size="-1">')
        result.push(%Q(#{get_i18n_term("version_of_schema")}: #{configVersion}))
        result.push('</font></p>')

        result.push('<div markdown="0">')
        result.push(format_examples(nil, input))

        if ( get_hash_value(input, "properties") )
           then
            result.push('<ul class="resources">')
            input['properties'].sort.to_h.each do |key, value|
                _primaryLanguage = nil
                _fallbackLanguage = nil

                _primaryLanguage = get_hash_value(input,  'i18n', @context.registers[:page]["lang"], 'properties', key)
                if ( @context.registers[:page]["lang"] == 'en' )
                    fallbackLanguageName = 'ru'
                else
                    fallbackLanguageName = 'en'
                end
                _fallbackLanguage = get_hash_value(input,  'i18n', fallbackLanguageName, 'properties', key)

                result.push(format_schema(key, value, input, _primaryLanguage, _fallbackLanguage, ["parameters"], resourceName, versionAPI ))
            end
            result.push('</ul>')
        end
        result.push('</div>')
        result.join
    end

    def format_cluster_configuration(input)
        result = []

        if ( @context.registers[:page]["lang"] == 'en' )
            fallbackLanguageName = 'ru'
        else
            fallbackLanguageName = 'en'
        end

        result.push('<div markdown="0">')
        result.push(CONVERTER.convert('## '+ input["kind"]))

        for i in 0..(input["apiVersions"].length-1)
          result.push("<p><font size='-1'>Version: #{input["apiVersions"][i]["apiVersion"]}</font></p>")
          item=input["apiVersions"][i]["openAPISpec"]
          item["APIversion"] = input["apiVersions"][i]["apiVersion"]
          item["resourceName"] = input["kind"]
          item["i18n"]={}
          item["i18n"]["ru"]=get_hash_value(input,"i18n","ru","apiVersions",i,"openAPISpec")
          item["i18n"]["en"]=get_hash_value(input,"apiVersions",i,"openAPISpec")

          if get_hash_value(item, 'description')
             if get_hash_value(item['i18n'][@context.registers[:page]["lang"]],"description") then
                 result.push(CONVERTER.convert(get_hash_value(item['i18n'][@context.registers[:page]["lang"]],"description")))
             elsif get_hash_value(item['i18n'][fallbackLanguageName],"description") then
                 result.push(CONVERTER.convert(item['i18n'][fallbackLanguageName]["description"]))
             else
                 result.push(CONVERTER.convert(item["description"]))
             end
          end

          result.push(format_configuration(item))
        end
        result.push('</div>')
        result.join
    end
  end
end

Liquid::Template.register_filter(Jekyll::Openapi)
