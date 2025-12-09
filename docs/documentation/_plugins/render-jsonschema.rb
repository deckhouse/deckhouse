require 'erb'
require 'cgi'
require 'uri'

module JSONSchemaRenderer
  class JSONSchemaRenderer
    # Liquid Template characters to escape
    @@CHARS_TO_ESCAPE = {
      '{' => '&#123;',
      '}' => '&#125;',
      '%' => '&#37;'
    }

    def convert(content)
      if @converter.nil?
        @converter = @site.find_converter_instance(::Jekyll::Converters::Markdown)
      end
      return @converter.convert(content)
    end

    def escape_chars(input)
      @@CHARS_TO_ESCAPE.each do |char, escaped_char|
        input = input.gsub(char, escaped_char)
      end
      input
    end

    # TODO: Refactor this according to the new data structure - x-doc-d8Editions instead of x-doc-d8Revision
    # moduleName
    # revision - Deckhouse revision
    # resourceType - can be 'crd' or 'moduleConfig' or 'clusterConfig'
    # resourceName - name of a CRD or an empty string
    # parameterName - name of a parameter
    # linkAnchor - HTML anchor for parameter
    def addRevisionParameter(moduleName, revision, resourceType, resourceName, parameterName, linkAnchor)
        item = Hash.new
        item['linkAnchor'] = linkAnchor
        item['resourceType'] = resourceType
        item['title'] = %Q(#{if resourceType == 'crd' and  resourceName then resourceName + ":&nbsp;" end}#{parameterName})
        if get_hash_value(@site.data['modules'], 'all', moduleName, %Q(parameters-#{revision})) == nil then
          if ! get_hash_value(@site.data['modules'], 'all', moduleName) then
            puts "NOTE: No modules data for module " + moduleName
          else
            @site.data['modules']['all'][moduleName][%Q(parameters-#{revision})] = Hash.new
          end
        end
        if get_hash_value(@site.data['modules'], 'all', moduleName) && get_hash_value(@site.data['modules'], 'all', moduleName, %Q(parameters-#{revision}),
           %Q(#{if resourceType != 'moduleConfig' then
                   if resourceName then resourceName + "." end
                end
               }#{parameterName})) == nil then
          @site.data['modules']['all'][moduleName][%Q(parameters-#{revision})][%Q(#{
            if resourceType != 'moduleConfig' then
                   if resourceName then resourceName + "." end
            end
            }#{parameterName})] = Hash.new
          @site.data['modules']['all'][moduleName][%Q(parameters-#{revision})][%Q(#{
            if resourceType != 'moduleConfig' then
                   if resourceName then resourceName + "." end
            end
            }#{parameterName})] = item
        else
          # Duplicate parameter. It may be because of the different api version on the same resource. Just skip it
        end
    end

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

    def get_i18n_parameter(name, primaryLanguage, fallbackLanguage, source=nil)
        if get_hash_value(primaryLanguage, name) then
            result = primaryLanguage[name]
        elsif get_hash_value(fallbackLanguage, name) then
            result = fallbackLanguage[name]
        elsif get_hash_value(source, name) then
            result = source[name]
        else
            result = ''
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
        version = a.scan(/.*v([1-9]+[0-9]*)((alpha|beta)([1-9]+[0-9]*))?$/).flatten
        aVersion = {"majVersion" => version[0].to_i , "stability" => convertAPIVersionChannelToInt(version[2]), "minVersion" => version[3].to_i ? version[3].to_i : 3 }
        version = b.scan(/.*v([1-9]+[0-9]*)((alpha|beta)([1-9]+[0-9]*))?$/).flatten
        bVersion = {"majVersion" => version[0].to_i , "stability" => convertAPIVersionChannelToInt(version[2]), "minVersion" => version[3].to_i ? version[3].to_i : 3 }
        return  aVersion["stability"] <=> bVersion["stability"] if aVersion["stability"] != bVersion["stability"]
        return  aVersion["majVersion"] <=> bVersion["majVersion"] if aVersion["minVersion"] == bVersion["minVersion"]
        return  aVersion["majVersion"] <=> bVersion["majVersion"] if aVersion["majVersion"] != bVersion["majVersion"]
        aVersion["minVersion"] <=> bVersion["minVersion"]
    end

    def AppendResource2Search(name, moduleName, url, resourceName, description, version = '', search = '')
        # Data for search index
        if name and name.length > 0
            searchItemData = Hash.new
            searchItemData['name'] = name
            searchItemData['module'] = moduleName.nil? ? '' : moduleName
            searchItemData['url'] = sprintf('%s#%s', url, name.downcase)
            searchItemData['resourceName'] = resourceName if resourceName
            searchItemData['isResource'] = true
            searchItemData['content'] = description if description
            searchItemData['version'] = version if version
            searchItemData['search'] = search if search

            addItemToIndex = true
            if !searchItemData['version'].nil?
                itemIsMoreStable = false
                otherVersions = @site.data['search']['searchItems'][@lang].
                   select { |item| item['url'] == searchItemData['url'] and item['name'] == searchItemData['name'] }
                if otherVersions and otherVersions.length > 0
                    addItemToIndex = false
                    otherVersions.each do | item |
                           if !item['version'].nil? and compareAPIVersion(searchItemData['version'], item['version']) == 1
                              # current item (searchItemData) has more stable version than we already have in array
                              itemIsMoreStable = true
                              addItemToIndex = true
                           end
                       end
                    if itemIsMoreStable
                        # delete items of the parameter from index? as they are less stable
                        @site.data['search']['searchItems'][@lang].
                          reject! { |item| item['url'] == searchItemData['url'] and item['name'] == searchItemData['name'] }
                    end
                end
            end

            @site.data['search']['searchItems'][@lang] << searchItemData if addItemToIndex == true

        end

    end


    def get_i18n_term(term)
        lang = @lang
        i18n = @site.data["i18n"]["common"]

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

    def get_search_keywords(primaryLanguage, fallbackLanguage = nil)
      return '' if !primaryLanguage
      if get_hash_value(primaryLanguage, "x-doc-search") then
          result = primaryLanguage["x-doc-search"]
      elsif get_hash_value(fallbackLanguage, "x-doc-search") then
          result = fallbackLanguage["x-doc-search"]
      else
          result = ''
      end

      if !result || result.length < 3
        result = ''
      end

      result.strip
    end

    def format_type(first_type, second_type)
        lang = @lang
        i18n = @site.data["i18n"]["common"]

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
          if attributes.has_key?('x-doc-examples')
              exampleKeyToUse = 'x-doc-examples'
          elsif attributes.has_key?('x-doc-example')
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
            if exampleKeyToUse == 'x-examples' or exampleKeyToUse == 'x-doc-examples' then
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
                            if exampleObject.is_a?(String) and exampleObject =~ /\`\`\`|\n/
                                exampleContent = "#{exampleObject}"
                            elsif attributes['type'] == 'boolean' then
                                exampleContent = %Q(```yaml\n#{(if name then {name => (exampleObject and true)} else (exampleObject and true) end).to_yaml.sub(/^---(\n| ){1}/,'')}```)
                            elsif attributes['type'] == 'integer' or attributes['type'] == 'number' then
                                exampleContent = %Q(```yaml\n#{(if name then {name => exampleObject.to_i} else exampleObject.to_i end).to_yaml.sub(/^---(\n| ){1}/,'')}```)
                            else
                                exampleContent = %Q(```yaml\n#{(if name then {name => exampleObject.to_s} else exampleObject.to_s end).to_yaml.sub(/^---(\n| ){1}/,'')}```)
                            end

                        end
            exampleContent = escape_chars(convert(exampleContent)).delete_prefix('<p>').sub(/<\/p>[\s]*$/,"")
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
        lang = @lang
        editionsString = ''

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
            # result.push(convert('**' + get_i18n_term('not_required_value_sentence')  + '**'))
        end

        if attributes.has_key?('x-doc-d8Editions')
          editionsString = %Q(<p><strong>#{@site.data['i18n']['common']['module_available_editions_prefix'][lang]}: #{
                attributes['x-doc-d8Editions']
                  # Filter editions present in module-editions
                  .select { |edition| @site.data['modules']['editions-addition'].key?(edition.sub('+', '-plus')) }
                  # Skip edition with language defined in _data/modules/editions-addition.yml and if it is not the current language.
                  .select {
                    |edition|
                      ! @site.data['modules']['editions-addition'][edition.sub('+', '-plus')]['languages'] or
                      @site.data['modules']['editions-addition'][edition.sub('+', '-plus')]['languages'].include?(lang)
                  }
                  # Sort by edition weight (_data/modules/editions-weight.yml)
                  .sort_by{
                    |edition|
                      weight = @site.data['modules']['editions-weight'][edition.sub('+', '-plus')]
                      weight.nil? ? Float::INFINITY : weight
                  }
                  # Map edition to titles
                  .map{
                    |edition|
                      editionData = @site.data['modules']['editions-addition'][edition.sub('+', '-plus')]
                      # The condition will always be true, as we selected only editions present in module-editions, but let it be
                      if editionData
                        if editionData['name_version']
                          editionData['name_version']
                        elsif editionData['name']
                          editionData['name']
                        else
                          puts "[WARN] No edition name for '#{edition}'"
                        end
                      else
                        puts "[WARN] No edition '#{edition}' (parameter - #{name}, parent - #{parent})"
                      end
                  }.join(', ')
                }</strong></p>)
        elsif attributes.has_key?('x-doc-d8Revision') # Deprecated!
          case attributes['x-doc-d8Revision']
          when "ee"
            editionsString = %Q(<p><strong>#{@site.data['i18n']['features']['ee'][lang].capitalize}</strong></p>)
          end
        end

        if attributes['description']
          result.push(sprintf(%q(<div class="resources__prop_description">%s%s</div>),editionsString,escape_chars(convert(get_i18n_description(primaryLanguage, fallbackLanguage, attributes)))))

        elsif editionsString and editionsString.size > 0
          result.push(sprintf(%q(<div class="resources__prop_description">%s</div>),editionsString))
        end

        if attributes.has_key?('x-doc-default')
            if attributes['x-doc-default'].is_a?(Array) or attributes['x-doc-default'].is_a?(Hash)
                if !( attributes['x-doc-default'].is_a?(Hash) and attributes['x-doc-default'].length < 1 )
                    result.push(sprintf(%q(<p class="resources__attrs"><span class="resources__attrs_name">%s:</span> <span class="resources__attrs_content"><code>%s</code></span></p>), get_i18n_term("default_value").capitalize, attributes['x-doc-default'].to_json))
                end
            else
                if attributes['type'] == 'string'
                    result.push(sprintf(%q(<p class="resources__attrs"><span class="resources__attrs_name">%s:</span> <span class="resources__attrs_content"><code>%s</code></span></p>), get_i18n_term("default_value").capitalize, attributes['x-doc-default']))
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
                    result.push(sprintf(%q(<p class="resources__attrs"><span class="resources__attrs_name">%s:</span> <span class="resources__attrs_content"><code>%s</code></span></p>), get_i18n_term("default_value").capitalize, attributes['default']))
                else
                    result.push(sprintf(%q(<p class="resources__attrs"><span class="resources__attrs_name">%s:</span> <span class="resources__attrs_content"><code>%s</code></span></p>), get_i18n_term("default_value").capitalize, attributes['default']))
                end
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
            result.push(convert(valuesRange.to_s))
        end

        if attributes.has_key?('enum')
            enum_result = '<p class="resources__attrs"><span class="resources__attrs_name">' + get_i18n_term("allowed_values").capitalize
            if name == "" and parent['type'] == 'array'
                enum_result += ' ' + get_i18n_term("allowed_values_of_array")
            end
            enum_result += ':</span> <span class="resources__attrs_content">'

            if attributes.has_key?('x-enum-descriptions') && attributes['x-enum-descriptions'].is_a?(Array) &&
               attributes['x-enum-descriptions'].size == attributes['enum'].size
                # Render enum values matched with descriptions
                enum_values = []
                descriptions = get_i18n_parameter('x-enum-descriptions', primaryLanguage, fallbackLanguage, attributes)
                attributes['enum'].each_with_index do |enum_value, index|
                    description = descriptions[index]
                    enum_values << "<code>#{enum_value}</code> — #{description}"
                end
                enum_result += enum_values.join('<br/>')
            else
                # If no descriptions, render just enum values
                enum_result += [*attributes['enum']].map { |e| "<code>#{e}</code>" }.join(', ')
            end

            enum_result += '</span></p>'
            result.push(enum_result)
        end

        if attributes.has_key?('pattern')
            result.push(sprintf(%q(<p class="resources__attrs"><span class="resources__attrs_name">%s:</span> <code class="resources__attrs_content">%s</code></p>),get_i18n_term("pattern").capitalize, CGI.escapeHTML(attributes['pattern']) ))
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
                description = %Q(<p class="resources__attrs"><span class="resources__attrs_name">#{get_i18n_term(caption).capitalize}:</span> <span class="resources__attrs_content"><code>#{lengthValue}</code></span></p>)
                result.push(convert(description.to_s))
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
    def format_schema(name, attributes, parent, primaryLanguage = nil, fallbackLanguage = nil, ancestors = [], resourceName = '', versionAPI = '', moduleName = '')
        result = Array.new()
        ancestorsPathString = ''

        if name.nil? or name == ''
            fullPath = ancestors + ['element']
            parameterTitle = get_i18n_term('element_of_array').capitalize
        else
            fullPath = ancestors + [name]
            parameterTitle = name
            ancestorsPathString = ancestors.slice(1, ancestors.length-1).join('.') + '.' if ancestors.length > 1
        end

        # The replacement with sub is for preserving anchor links for ModuleConfig parameters
        linkAnchor = fullPath.join('-').downcase.sub(/^parameters-settings-/, 'parameters-')
        # URL-encode linkAnchor for proper URL fragment handling (similar to Hugo's urlquery)
        linkAnchor = URI.encode_www_form_component(linkAnchor)
        pathString = fullPath.slice(1,fullPath.length-1).join('.')

        # Data for search index
        if name and name.length > 0 and ! @site.data['search']['skipParameters'].include?(name)
            searchItemData = Hash.new
            searchItemData['pathString'] = pathString
            searchItemData['name'] = name
            searchItemData['module'] = moduleName.nil? ? '' : moduleName
            searchItemData['url'] = sprintf(%q(%s#%s), @page["url"].sub(%r{^(/?ru/|/?en/)}, ''), linkAnchor)
            searchItemData['deprecated'] = get_hash_value(attributes,"deprecated") or get_hash_value(attributes,"x-doc-deprecated") ? true : false
            searchItemData['version'] = versionAPI
            searchItemData['resourceName'] = resourceName
            searchItemData['content'] = convert(get_i18n_description(primaryLanguage, fallbackLanguage, attributes)).to_s if attributes['description']
            searchKeywords = get_search_keywords(primaryLanguage, fallbackLanguage)
            searchItemData['search'] = searchKeywords if searchKeywords

            addItemToIndex = true
            if !searchItemData['version'].nil?
                itemIsMoreStable = false
                otherVersions = @site.data['search']['searchItems'][@lang].
                   select { |item| item['pathString'] == searchItemData['pathString'] and item['resourceName'] == searchItemData['resourceName'] }
                if otherVersions and otherVersions.length > 0
                    addItemToIndex = false
                    otherVersions.each do | item |
                           if !item['version'].nil? and compareAPIVersion(searchItemData['version'], item['version']) == 1
                              # current item (searchItemData) has more stable version than we already have in array
                              itemIsMoreStable = true
                              addItemToIndex = true
                           end
                       end
                    if itemIsMoreStable
                        # delete items of the parameter from index? as they are less stable
                        @site.data['search']['searchItems'][@lang].
                          reject! { |item| item['pathString'] == searchItemData['pathString'] and item['resourceName'] == searchItemData['resourceName'] }
                    end
                end
            end

            @site.data['search']['searchItems'][@lang] << searchItemData if addItemToIndex
        end

        if parameterTitle != ''
            parameterTextContent = ''
            result.push('<li>')
            result.push('<div class="resources__prop_wrap">')
            attributesType = ''
            if attributes.is_a?(Hash)
              if attributes.has_key?('type')
                 attributesType = attributes["type"]
              elsif attributes.has_key?('x-kubernetes-int-or-string')
                 attributesType = "x-kubernetes-int-or-string"
              end
            end

            if ( get_hash_value(attributes, 'x-doc-deprecated') or get_hash_value(attributes, 'deprecated') )
                parameterTextContent = sprintf(%q(<div id="%s" data-anchor-id="%s" class="resources__prop_name anchored">
                    <span class="plus-icon"><svg xmlns="http://www.w3.org/2000/svg" width="10" height="10" viewBox="0 0 10 10" fill="none"><path d="M5.00005 1.5V4.99995M5.00005 4.99995V8.5M5.00005 4.99995H1.5M5.00005 4.99995H8.5" stroke="#0D69F2" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg></span>
                    <span  class="minus-icon"><svg xmlns="http://www.w3.org/2000/svg" width="10" height="8" viewBox="0 0 10 8" fill="none"><path d="M1.5 3.99982L8.5 3.99982" stroke="#0D69F2" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg></span>
                    <div><span class="ancestors">%s</span><span>%s</span></div><span title="%s" class="resources__prop_is_deprecated">%s</span></div>), linkAnchor, linkAnchor, ancestorsPathString, parameterTitle,get_i18n_term('deprecated_parameter_hint'), get_i18n_term('deprecated_parameter') )
            else
                parameterTextContent = sprintf(%q(<div id="%s" data-anchor-id="%s" class="resources__prop_name anchored">
                    <span class="plus-icon"><svg xmlns="http://www.w3.org/2000/svg" width="10" height="10" viewBox="0 0 10 10" fill="none"><path d="M5.00005 1.5V4.99995M5.00005 4.99995V8.5M5.00005 4.99995H1.5M5.00005 4.99995H8.5" stroke="#0D69F2" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg></span>
                    <span  class="minus-icon"><svg xmlns="http://www.w3.org/2000/svg" width="10" height="8" viewBox="0 0 10 8" fill="none"><path d="M1.5 3.99982L8.5 3.99982" stroke="#0D69F2" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg></span>
                    <div><span class="ancestors">%s</span><span>%s</span></div></div>), linkAnchor, linkAnchor, ancestorsPathString, parameterTitle)
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

        if (@moduleName != '') and attributes.has_key?('x-doc-d8Revision')
          case attributes['x-doc-d8Revision']
          when "ee"
            addRevisionParameter(@moduleName, 'ee', @resourceType, resourceName, name, linkAnchor)
          end
        end

        result.push('</div>')

        if attributes.is_a?(Hash) and attributes.has_key?("properties")
            result.push('<ul>')
            attributes["properties"].sort.to_h.each do |key, value|
                result.push(format_schema(key, value, attributes, get_hash_value(primaryLanguage, "properties", key), get_hash_value(fallbackLanguage, "properties", key), fullPath, resourceName, versionAPI, moduleName))
            end
            result.push('</ul>')
        elsif attributes.is_a?(Hash) and  attributes.has_key?('items')
            if get_hash_value(attributes,'items','properties')
                #  Array of objects
                result.push('<ul>')
                attributes['items']["properties"].sort.to_h.each do |item_key, item_value|
                    result.push(format_schema(item_key, item_value, attributes['items'], get_hash_value(primaryLanguage,"items", "properties", item_key) , get_hash_value(fallbackLanguage,"items", "properties", item_key), fullPath, resourceName, versionAPI, moduleName))
                end
                result.push('</ul>')
            else
                # Array of non-objects (string, integer, etc.)
                keysToShow = ['description', 'example', 'x-examples', 'x-doc-example', 'x-doc-examples', 'enum', 'default', 'x-doc-default', 'minimum', 'maximum', 'pattern', 'minLength', 'maxLength']
                if (attributes['items'].keys & keysToShow).length > 0
                    lang = @lang
                    i18n = @site.data["i18n"]["common"]
                    result.push('<ul>')
                    result.push(format_schema(nil, attributes['items'], attributes, get_hash_value(primaryLanguage,"items") , get_hash_value(fallbackLanguage,"items"), fullPath, resourceName, versionAPI, moduleName))
                    result.push('</ul>')
                end
            end
        else
            # result.push("no properties for #{name}")
        end

        # Render additionalProperties if they exist
        if attributes.is_a?(Hash) and attributes.has_key?('additionalProperties')
            additionalProps = attributes['additionalProperties']
            
            # Render additionalProperties if it is a schema object with properties (not primitive type) AND parent has no properties
            if additionalProps.is_a?(Hash) and 
               (not attributes.has_key?('properties') or attributes['properties'].nil?) and
               additionalProps.has_key?('properties') and
               (not additionalProps.has_key?('type') or additionalProps['type'] == 'object')
                additionalPropsData = additionalProps.dup
                additionalPropsLangData = get_hash_value(primaryLanguage, 'additionalProperties')
                additionalPropsFallbackLangData = get_hash_value(fallbackLanguage, 'additionalProperties')
                additionalPropsRequired = get_hash_value(additionalPropsData, 'required')
                
                # Prepare the description with special text for additionalProperties object
                additionalPropertyName = '<KEY_NAME>'.gsub('<', '&lt;').gsub('>', '&gt;')
                additionalPropertyNameQuoted = '`<KEY_NAME>`'
                mapKeyName = get_hash_value(additionalPropsData, 'x-doc-map-key-name')
                additionalPropertyNameLang = get_i18n_term('additional_property_name')
                
                if mapKeyName
                    specialDescriptionText = "#{additionalPropertyNameQuoted} — #{mapKeyName}"
                else
                    specialDescriptionText = "#{additionalPropertyNameQuoted} — #{additionalPropertyNameLang}."
                end
                
                # Get existing description if any
                existingDescription = ''
                if get_hash_value(additionalPropsLangData, 'description')
                    existingDescription = additionalPropsLangData['description']
                elsif get_hash_value(additionalPropsData, 'description')
                    existingDescription = additionalPropsData['description']
                end
                
                # Combine special text with existing description
                finalDescription = specialDescriptionText
                if existingDescription and existingDescription.length > 0
                    finalDescription = "#{specialDescriptionText}\n\n#{existingDescription}"
                end
                
                # Create modified data with updated description
                additionalPropsData['description'] = finalDescription
                if additionalPropsLangData
                    additionalPropsLangData = additionalPropsLangData.dup
                    additionalPropsLangData['description'] = finalDescription
                else
                    additionalPropsLangData = { 'description' => finalDescription }
                end
                
                result.push('<ul>')
                result.push(format_schema(additionalPropertyName, additionalPropsData, attributes, additionalPropsLangData, additionalPropsFallbackLangData, fullPath, resourceName, versionAPI, moduleName))
                result.push('</ul>')
            # Only render if additionalProperties is a schema object AND has properties (normal case when parent has properties)
            elsif additionalProps.is_a?(Hash) and additionalProps.has_key?('properties')
                additionalPropsData = additionalProps
                additionalPropsLangData = get_hash_value(primaryLanguage, 'additionalProperties')
                additionalPropsFallbackLangData = get_hash_value(fallbackLanguage, 'additionalProperties')
                additionalPropsRequired = get_hash_value(additionalPropsData, 'required')
                result.push('<ul>')
                result.push(format_schema('additionalProperties', additionalPropsData, attributes, additionalPropsLangData, additionalPropsFallbackLangData, fullPath, resourceName, versionAPI, moduleName))
                result.push('</ul>')
            end
            
        end

        # Render patternProperties if they exist
        if attributes.is_a?(Hash) and attributes.has_key?('patternProperties')
            attributes['patternProperties'].each do |pattern, patternSchema|
                if patternSchema.is_a?(Hash)
                    # Get language data for pattern
                    patternLangData = {}
                    if primaryLanguage and primaryLanguage.is_a?(Hash) and primaryLanguage.has_key?('patternProperties')
                        indexedLangData = get_hash_value(primaryLanguage, 'patternProperties', pattern)
                        if indexedLangData and indexedLangData.is_a?(Hash)
                            patternLangData = indexedLangData
                        end
                    end
                    patternFallbackLangData = {}
                    if fallbackLanguage and fallbackLanguage.is_a?(Hash) and fallbackLanguage.has_key?('patternProperties')
                        indexedLangData = get_hash_value(fallbackLanguage, 'patternProperties', pattern)
                        if indexedLangData and indexedLangData.is_a?(Hash)
                            patternFallbackLangData = indexedLangData
                        end
                    end
                    patternRequired = get_hash_value(patternSchema, 'required')
                    
                    # Use pattern in path and display - ADD SLASHES FOR REGEX
                    patternName = pattern
                    patternNameQuoted = "`/#{pattern}/`"
                    patternNameForPath = "/#{pattern}/"
                    
                    # Handle objects with properties
                    if patternSchema.has_key?('properties') and (not patternSchema.has_key?('type') or patternSchema['type'] == 'object')
                        # Prepare the description with special text for patternProperties object
                        mapKeyName = get_hash_value(patternSchema, 'x-doc-map-key-name')
                        patternPropertyNameLang = get_i18n_term('pattern_property_name')
                        
                        if mapKeyName
                            specialDescriptionText = "#{patternNameQuoted} — #{mapKeyName}"
                        else
                            specialDescriptionText = "#{patternNameQuoted} — #{patternPropertyNameLang}."
                        end
                        
                        # Get existing description if any
                        existingDescription = ''
                        if patternLangData.is_a?(Hash) and patternLangData.has_key?('description')
                            existingDescription = patternLangData['description']
                        elsif patternSchema.has_key?('description')
                            existingDescription = patternSchema['description']
                        end
                        
                        # Combine special text with existing description
                        finalDescription = specialDescriptionText
                        if existingDescription and existingDescription.length > 0
                            finalDescription = "#{specialDescriptionText}\n\n#{existingDescription}"
                        end
                        
                        # Create modified data with updated description
                        modifiedPatternSchema = patternSchema.dup
                        modifiedPatternSchema['description'] = finalDescription
                        if patternLangData.is_a?(Hash) and patternLangData.length > 0
                            modifiedPatternLangData = patternLangData.dup
                            modifiedPatternLangData['description'] = finalDescription
                            patternLangData = modifiedPatternLangData
                        else
                            patternLangData = { 'description' => finalDescription }
                        end
                        
                        result.push('<ul>')
                        result.push(format_schema(patternNameForPath, modifiedPatternSchema, attributes, patternLangData, patternFallbackLangData, fullPath, resourceName, versionAPI, moduleName))
                        result.push('</ul>')
                    # Handle arrays - always render arrays
                    elsif patternSchema['type'] == 'array' or patternSchema.has_key?('items')
                        result.push('<ul>')
                        result.push(format_schema(patternNameForPath, patternSchema, attributes, patternLangData, patternFallbackLangData, fullPath, resourceName, versionAPI, moduleName))
                        result.push('</ul>')
                    # Handle primitive types and objects without properties
                    else
                        # Prepare the description with special text for patternProperties primitive
                        mapKeyName = get_hash_value(patternSchema, 'x-doc-map-key-name')
                        patternPropertyNameLang = get_i18n_term('pattern_property_name')
                        
                        if mapKeyName
                            specialDescriptionText = "#{patternNameQuoted} — #{mapKeyName}"
                        else
                            specialDescriptionText = "#{patternNameQuoted} — #{patternPropertyNameLang}."
                        end
                        
                        # Get existing description if any
                        existingDescription = ''
                        if patternLangData.is_a?(Hash) and patternLangData.has_key?('description')
                            existingDescription = patternLangData['description']
                        elsif patternSchema.has_key?('description')
                            existingDescription = patternSchema['description']
                        end
                        
                        # Combine special text with existing description
                        finalDescription = specialDescriptionText
                        if existingDescription and existingDescription.length > 0
                            finalDescription = "#{specialDescriptionText}\n\n#{existingDescription}"
                        end
                        
                        # Create modified data with updated description
                        modifiedPatternSchema = patternSchema.dup
                        modifiedPatternSchema['description'] = finalDescription
                        if patternLangData.is_a?(Hash) and patternLangData.length > 0
                            modifiedPatternLangData = patternLangData.dup
                            modifiedPatternLangData['description'] = finalDescription
                            patternLangData = modifiedPatternLangData
                        else
                            patternLangData = { 'description' => finalDescription }
                        end
                        
                        keysToShow = ['description', 'example', 'x-examples', 'x-doc-example', 'x-doc-examples', 'enum', 'default', 'x-doc-default', 'minimum', 'maximum', 'pattern', 'minLength', 'maxLength', 'type']
                        if (modifiedPatternSchema.keys & keysToShow).length > 0
                            result.push('<ul>')
                            result.push(format_schema(patternNameForPath, modifiedPatternSchema, attributes, patternLangData, patternFallbackLangData, fullPath, resourceName, versionAPI, moduleName))
                            result.push('</ul>')
                        end
                    end
                end
            end
        end

        if parameterTitle != ''
            result.push('</li>')
        end
        result.join
    end

    def format_crd(site, page, input, moduleName = "")
        return nil if !input

        @site = site
        @page = page
        @lang = page['lang']

        @moduleName = moduleName
        @resourceType = "crd"
        resourceName = ''
        resourceGroup = ''

        if ( @lang == 'en' )
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
                resourceGroup = get_hash_value(input,'metadata','name')
                fullPath = [sprintf(%q(v1beta1-%s), input["spec"]["names"]["kind"])]
                result.push("<h2>#{resourceName}</h2>")
                result.push('<p><font size="-1">Scope: ' + input["spec"]["scope"])
                if input["spec"].has_key?("version") then
                   result.push('<br/>Version: ' + input["spec"]["version"] + '</font></p>')
                end

                if get_hash_value(input,'spec','validation','openAPIV3Schema','description')
                   searchKeywords = get_search_keywords(input['i18n'][@lang],"spec","validation","openAPIV3Schema", input['i18n'][fallbackLanguageName],"spec","validation","openAPIV3Schema")

                   if get_hash_value(input['i18n'][@lang],"spec","validation","openAPIV3Schema","description") then
                       description = escape_chars(convert(get_hash_value(input['i18n'][@lang],"spec","validation","openAPIV3Schema","description")))
                       result.push(description)
                       AppendResource2Search(input["spec"]["names"]["kind"], @moduleName, @page["url"].sub(%r{^(/?ru/|/?en/)}, ''), '', description, searchKeywords)
                   elsif get_hash_value(input['i18n'][fallbackLanguageName],"spec","validation","openAPIV3Schema","description") then
                       description = escape_chars(convert(input['i18n'][fallbackLanguageName]["spec"]["validation"]["openAPIV3Schema"]["description"]))
                       result.push(description)
                       AppendResource2Search(input["spec"]["names"]["kind"], @moduleName, @page["url"].sub(%r{^(/?ru/|/?en/)}, ''), '', description, searchKeywords)
                   else
                       description = escape_chars(convert(input["spec"]["validation"]["openAPIV3Schema"]["description"]))
                       result.push(description)
                       AppendResource2Search(input["spec"]["names"]["kind"], @moduleName, @page["url"].sub(%r{^(/?ru/|/?en/)}, ''), '', description, searchKeywords)
                   end
                end

                if input["spec"]["validation"]["openAPIV3Schema"].has_key?('properties')
                    result.push('<ul class="resources">')
                    input["spec"]["validation"]["openAPIV3Schema"]['properties'].sort.to_h.each do |key, value|
                    _primaryLanguage = nil
                    _fallbackLanguage = nil

                    if  input['i18n'][@lang] then
                        _primaryLanguage = get_hash_value(input['i18n'][@lang],"spec","validation","openAPIV3Schema","properties",key)
                    end
                    if   input['i18n'][fallbackLanguageName] then
                        _fallbackLanguage = get_hash_value(input['i18n'][fallbackLanguageName],"spec","validation","openAPIV3Schema","properties",key)
                    end
                        result.push(format_schema(key, value, input["spec"]["validation"]["openAPIV3Schema"], _primaryLanguage, _fallbackLanguage, fullPath, resourceName, versionAPI, moduleName))
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

                 resourceName = input["spec"]["names"]["kind"]
                 resourceGroup = get_hash_value(input,'metadata','name')
                 result.push("<h2>#{resourceName}</h2>")

                 if  input["spec"]["versions"].length > 1 then
                     result.push('<p><font size="-1">Scope: ' + input["spec"]["scope"] + '</font></p>')
                     result.push('<div class="tabs">')
                     activeStatus=" active"
                     input["spec"]["versions"].sort{ |a, b| compareAPIVersion(a['name'],b['name']) }.reverse.each do |item|
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
                 input["spec"]["versions"].sort{ |a, b| compareAPIVersion(a['name'],b['name']) }.reverse.each do |item|
                    _primaryLanguage = nil
                    _fallbackLanguage = nil
                    versionAPI = item['name']

                    if input["spec"]["versions"].length == 1 then
                        result.push('<p><font size="-1">Scope: ' + input["spec"]["scope"])
                        result.push('<br/>Version: ' + item['name'] + '</font></p>')
                    else
                        #result.push(convert("### " + item['name'] + ' {#' + input["spec"]["names"]["kind"].downcase + '-' + item['name'].downcase + '}'))
                        #result.push('<p><font size="-1">Scope: ' + input["spec"]["scope"] + '</font></p>')
                    end

                    if input["spec"]["versions"].length > 1 then
                        result.push("<div id='%s_%s' class='tabs__content tabs__content__%s%s'>" %
                            [ input["spec"]["names"]["kind"].downcase, item['name'].downcase,
                            input["spec"]["names"]["kind"].downcase, activeStatus ])
                        activeStatus = ""
                    end

                     if get_hash_value(item,'deprecated') then
                         result.push(sprintf('<p><strong>%s</strong></p>',get_i18n_term('deprecated_resource')))
                     end

                    description = ''
                    if get_hash_value(item,'schema','openAPIV3Schema','description') then
                       if    input['i18n'][@lang] and
                             get_hash_value(input['i18n'][@lang],"spec","versions") and
                             input['i18n'][@lang]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0] then
                           description = convert(input['i18n'][@lang]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0]["schema"]["openAPIV3Schema"]["description"])
                           result.push(escape_chars(description))
                       elsif input['i18n'][fallbackLanguageName] and
                             get_hash_value(input['i18n'][fallbackLanguageName],"spec","versions") and
                             input['i18n'][fallbackLanguageName]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0] then
                           description = convert(input['i18n'][fallbackLanguageName]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0]["schema"]["openAPIV3Schema"]["description"])
                           result.push(escape_chars(description))
                       else
                           description = escape_chars(convert(item["schema"]["openAPIV3Schema"]["description"]))
                           result.push('<div class="resources__prop_description">' + description + '</div>')
                       end
                    end

                    # Get search keywords
                    if    get_hash_value(input['i18n'][@lang],"spec","versions") and
                          input['i18n'][@lang]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0] then
                          searchKeywords = get_search_keywords(input['i18n'][@lang]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0]["schema"]["openAPIV3Schema"])
                    elsif get_hash_value(input['i18n'][fallbackLanguageName],"spec","versions") and
                          input['i18n'][fallbackLanguageName]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0] then
                          searchKeywords = get_search_keywords(input['i18n'][fallbackLanguageName]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0]["schema"]["openAPIV3Schema"])
                    else
                          searchKeywords = get_search_keywords(item["schema"]["openAPIV3Schema"])
                    end

                    AppendResource2Search(input["spec"]["names"]["kind"],
                                          moduleName,
                                          @page["url"].sub(%r{^(/?ru/|/?en/)}, ''),
                                          input["spec"]["names"]["kind"],
                                          description,
                                          item['name'],
                                          searchKeywords)

                    if item["schema"]["openAPIV3Schema"].has_key?('x-doc-examples') or item["schema"]["openAPIV3Schema"].has_key?('x-doc-example') or
                       item["schema"]["openAPIV3Schema"].has_key?('example') or item["schema"]["openAPIV3Schema"].has_key?('x-examples')
                    then
                        result.push('<div markdown="0">')
                        result.push(format_examples(nil, item["schema"]["openAPIV3Schema"]))
                        result.push('</div>')
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

                        if  input['i18n'][@lang] and
                            get_hash_value(input['i18n'][@lang],"spec","versions") and
                            input['i18n'][@lang]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0]
                        then
                            _primaryLanguage = input['i18n'][@lang]["spec"]["versions"].select {|i| i['name'].to_s == item['name'].to_s; }[0]
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

                        result.push(format_schema(key, value, item['schema']['openAPIV3Schema'] , _primaryLanguage, _fallbackLanguage, fullPath, resourceName, versionAPI, moduleName))
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

        # Add CRD to the list of module resources.
        if resourceGroup && moduleName && site.data.dig('modules','all',moduleName) then
            site.data['modules']['all'][moduleName]['docs'] = {} if ! site.data['modules']['all'][moduleName].has_key?('docs')
            site.data['modules']['all'][moduleName]['docs']['crds'] = [] if ! site.data['modules']['all'][moduleName]['docs'].has_key?('crds')
            # Add CRD to the list of module resources.
            site.data['modules']['all'][moduleName]['docs']['crds'] = site.data['modules']['all'][moduleName]['docs']['crds'] | [resourceGroup]
            # Add CRD to the list of all resources.
            site.data['modules']['crds'] = {} if ! site.data['modules'].has_key?('crds')
            if ! site.data['modules']['crds'].has_key?(resourceGroup)
              site.data['modules']['crds'][resourceGroup] = {
                    'internal' => {
                      'en' => "/en/modules/%s/cr.html\#%s" % [ moduleName, resourceName.downcase ],
                      'ru' => "/ru/modules/%s/cr.html\#%s" % [ moduleName, resourceName.downcase ]
                    },
                    'external' => {
                      'en' => "/modules/%s/cr.html\#%s" % [ moduleName, resourceName.downcase ],
                      'ru' => "/modules/%s/cr.html\#%s" % [ moduleName, resourceName.downcase ]
                    }
              }
            end
        end

        result.join
    end

    #
    # Returns configuration module content from the openAPI spec
    def format_configuration(site, page, input, moduleName, moduleConfig = false)
        @site = site
        @page = page
        @lang = @page['lang']

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

        if moduleConfig then
            # ModuleConfiguration schema
            configVersion = 1
            if ( get_hash_value(input, "x-config-version") ) then
              configVersion = input['x-config-version']
            end
            result.push('<p><font size="-1">')
            result.push(%Q(#{get_i18n_term("version_of_schema")}: #{configVersion}))
            result.push('</font></p>')
            if !( get_hash_value(input, 'properties', 'settings' ) )
               input['properties'] = { "settings" => { "type" => "object", "properties" => input['properties'] } }
               input['properties']['settings']['required'] = input['required'] if input['required']
            end
            if !( get_hash_value(input, 'i18n', 'en', 'properties', 'settings' ) )
               input['i18n']['en']['properties'] = { "settings" => { "type" => "object", "properties" => input['i18n']['en']['properties'] } }
            end
            if !( get_hash_value(input, 'i18n', 'ru', 'properties', 'settings' ) ) and get_hash_value(input, 'i18n', 'ru', 'properties' )
               input['i18n']['ru']['properties'] = { "settings" => { "type" => "object", "properties" => input['i18n']['ru']['properties'] } }
            end
        end

        if input.has_key?('x-doc-examples') or input.has_key?('x-doc-example') or
           input.has_key?('example') or input.has_key?('x-examples')
        then
            result.push('<div markdown="0">')
            result.push(format_examples(nil, input))
            result.push('</div>')
        end


        if ( get_hash_value(input, "properties") )
        then
            result.push('<ul class="resources">')
            input['properties'].sort.to_h.each do |key, value|
                _primaryLanguage = nil
                _fallbackLanguage = nil

                _primaryLanguage = get_hash_value(input,  'i18n', @lang, 'properties', key)
                if ( @lang == 'en' )
                    fallbackLanguageName = 'ru'
                else
                    fallbackLanguageName = 'en'
                end
                _fallbackLanguage = get_hash_value(input,  'i18n', fallbackLanguageName, 'properties', key)
                ancestor = ( resourceName.nil? or resourceName.length < 1 ) ? "parameters" : resourceName
                result.push(format_schema(key, value, input, _primaryLanguage, _fallbackLanguage, [ancestor], resourceName, versionAPI, moduleName))
            end
            result.push('</ul>')
        end
        result.push('</div>')
        result.join
    end

    def format_cluster_configuration(site, page, input, moduleName = "")
        @site = site
        @page = page
        @lang = @page['lang']
        @moduleName = moduleName
        @resourceType = "clusterConfig"

        result = []

        if ( @lang == 'en' )
            fallbackLanguageName = 'ru'
        else
            fallbackLanguageName = 'en'
        end

        result.push('<div markdown="0">')
        result.push(convert(%Q(<h2>#{input["kind"]}</h2>)))

        for i in 0..(input["apiVersions"].length-1)
          result.push("<p><font size='-1'>Version: #{input["apiVersions"][i]["apiVersion"]}</font></p>")
          item=input["apiVersions"][i]["openAPISpec"]
          item["APIversion"] = input["apiVersions"][i]["apiVersion"]
          item["resourceName"] = input["kind"]
          item["i18n"]={}
          item["i18n"]["ru"]=get_hash_value(input,"i18n","ru","apiVersions",i,"openAPISpec")
          item["i18n"]["en"]=get_hash_value(input,"apiVersions",i,"openAPISpec")

          description = ''
          if get_hash_value(item, 'description')
             if get_hash_value(item['i18n'][@lang],"description") then
                 description = convert(get_hash_value(item['i18n'][@lang],"description"))
             elsif get_hash_value(item['i18n'][fallbackLanguageName],"description") then
                 description = convert(item['i18n'][fallbackLanguageName]["description"])
             else
                 description = convert(item["description"])
             end
             result.push(escape_chars(description))
          end

          searchKeywords = get_search_keywords(item['i18n'][@lang], item['i18n'][fallbackLanguageName])

          AppendResource2Search(item["resourceName"],
                                moduleName,
                                @page["url"].sub(%r{^(/?ru/|/?en/)}, ''),
                                item["resourceName"],
                                description,
                                item["APIversion"],
                                searchKeywords)

          result.push(format_configuration(@site, @page, item, moduleName, false))
        end
        result.push('</div>')
        result.join
    end

    def format_module_configuration(site, page, input, moduleName = "")
        return if input.nil? || input.empty? || input["properties"].nil?|| input["properties"].empty?

        @moduleName = moduleName
        @resourceType = "moduleConfig"

        format_configuration(site, page, input, moduleName, true)
    end
  end
end
