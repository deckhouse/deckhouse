# Copyright 2025 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

require 'find'
require 'yaml'

PATH_TO_SIDEBARS = "/srv/jekyll-data/site/_data/sidebars"
PATH_TO_DATA = "/srv/jekyll-data/site/_data"

def check_urls(folders)
  result = false
  folders.each_with_index do |entity, idx|
    if entity.key?('url')
      result = true
    end
  end
  result
end

def read_folders(folders, title_en, title_ru, result)
    if folders.is_a?(Array)
         folders.each do |entry|
            # Проверяем, что в entry есть ключ 'folders' и это массив
            if entry.is_a?(Hash) && entry['folders'].is_a?(Array)
              read_folders(entry['folders'], entry['title']['en'], entry['title']['ru'], result)
            end
            if check_urls(folders)
                url = folders[0]['url']
                if url != nil
                    section_url = url.split('/')[0...-1].join('/')
                    result.push("#{section_url}:")
                    result.push("  title:")
                    result.push("    en: #{title_en}")
                    result.push("    ru: #{title_ru}")
                    end
                end
          end
        end
end


Find.find(PATH_TO_SIDEBARS) do |file|
  next unless File.file?(file)

  if file.end_with?(".yml")
    name = File.basename(file, ".*")

    puts "Preparing breadcrumbs for #{name.capitalize}..."

    if name == "virtualization-platform"
      name = "dvp"
    end

    output_file = File.join(PATH_TO_DATA, name, "breadcrumbs.yml")
    output_dir = File.join(PATH_TO_DATA, name)
    if !Dir.exist?(output_dir)
      Dir.mkdir(output_dir)
      end
    result = []

    begin
      data = YAML.load_file(file)
      rescue Psych::SyntaxError => e
        puts "YAML syntax error: #{e.message}"
      rescue Errno::ENOENT
        puts "File not found: #{file}"
    end

    if data.is_a?(Hash) && data['entries'].is_a?(Array)
      data['entries'].each do |entry|
        # Проверяем, что в entry есть ключ 'folders' и это массив
        if entry.is_a?(Hash) && entry['folders'].is_a?(Array)
          # По желанию выведем содержимое folders
          read_folders(entry['folders'], entry['title']['en'], entry['title']['ru'], result)
        end
      end
    end

    File.write(output_file, result.join("\n"))

  end
end

