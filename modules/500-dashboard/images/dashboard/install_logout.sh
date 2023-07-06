#!/bin/sh -e

# Copyright 2023 Flant JSC
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

hash=$(grep -Eo 'en\.main\.(.*)\.js' /public/en/index.html | cut -d '.' -f 3)

cat /public/logout_button.js >> "/public/de/de.main.${hash}.js"
cat /public/logout_button.js >> "/public/en/en.main.${hash}.js"
cat /public/logout_button.js >> "/public/es/es.main.${hash}.js"
cat /public/logout_button.js >> "/public/fr/fr.main.${hash}.js"
cat /public/logout_button.js >> "/public/ja/ja.main.${hash}.js"
cat /public/logout_button.js >> "/public/ko/ko.main.${hash}.js"
cat /public/logout_button.js >> "/public/zh-Hans/zh-Hans.main.${hash}.js"
cat /public/logout_button.js >> "/public/zh-Hant/zh-Hant.main.${hash}.js"
cat /public/logout_button.js >> "/public/zh-Hant-HK/zh-Hant-HK.main.${hash}.js"
