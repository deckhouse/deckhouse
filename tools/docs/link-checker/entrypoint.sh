#!/usr/bin/env sh

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

exit_code=0

cd /src/en
echo "Checking links (EN)"
htmlproofer --allow_missing_href --allow-hash-href --ignore-missing-alt --ignore-empty-alt \
       --ignore-urls '/^(.+deckhouse\.io)?\/privacy-policy(\/|\.html)$/,/^(.+deckhouse\.(io|ru))?\/security-policy\.html/,/^(.+deckhouse\.(io|ru))?\/products\/kubernetes-platform\/modules\/.*$/,/^(.+deckhouse\.(io|ru))?\/modules\/.*$/,/\.sslip\.io/,/^\/[^/.]+\.(svg|png|webmanifest|ico)$/,/^\/downloads\/deckhouse-cli.+\//,/\/(terms-of-service|success-stories|deckhouse-vs-kaas|services|tech-support|security|cookie-policy|community|regulations|license-rules|license|security-policy|webinars|news|education-license|partners-program|how-to-buy|moving-from-openshift|partnership-products|academy|)\/.*/,/^\/products\/(delivery-kit|stronghold|commander|observability-platform)\//,/^\/products\/enterprise_edition\.html/,/^\/products\/kubernetes-platform\/pricing\/.*/,/localhost/,/https\:\/\/t.me/,/docs-prv\.pcisecuritystandards\.org/,/gitlab.com\/profile/,/dash.cloudflare.com\/profile/,/example.com/,/vmware.com/,/.slack.com/,/habr.com/,/flant.ru/,/bcrypt-generator.com/,/candi\/bashible\/bashbooster/,/..\/..\/compare\//,/compare\/ru\//,/compare\/en\//,/\.yml$/,/\.yaml$/,/\.tmpl$/,/\.tpl$/,/^\/products\/kubernetes-platform\/$/,/^\/products\/virtualization-platform\/$/' \
       --swap-urls "https\:\/\/deckhouse\.io\/guides\/:/products/kubernetes-platform/guides/,https\:\/\/deckhouse\.io\/gs\/:/products/kubernetes-platform/gs/,https\:\/\/deckhouse\.io\/:/,\/documentation\/latest\/:/documentation/,\/documentation\/v1\/:/documentation/" \
       --ignore-files "404.html" --ignore-status-codes "0,429" .

if [ $? -ne 0 ]; then exit_code=$?; fi

cd /src/ru
echo -e "\n\nChecking links (RU)"
htmlproofer --allow_missing_href --allow-hash-href --ignore-missing-alt --ignore-empty-alt \
       --allow_missing_href --allow-hash-href --ignore-missing-alt --ignore-empty-alt \
       --ignore-urls '/^(.+deckhouse\.io)?\/privacy-policy(\/|\.html)$/,/^(.+deckhouse\.(io|ru))?\/security-policy\.html/,/^(.+deckhouse\.(io|ru))?\/products\/kubernetes-platform\/modules\/.*$/,/^(.+deckhouse\.(io|ru))?\/modules\/.*$/,/\.sslip\.io/,/^\/[^/.]+\.(svg|png|webmanifest|ico)$/,/^\/downloads\/deckhouse-cli.+\//,/\/(terms-of-service|success-stories|deckhouse-vs-kaas|services|tech-support|security|cookie-policy|community|regulations|license-rules|license|security-policy|webinars|news|education-license|partners-program|how-to-buy|moving-from-openshift|partnership-products|academy|)\/.*/,/^\/products\/(delivery-kit|stronghold|commander|observability-platform)\//,/^\/products\/enterprise_edition\.html/,/^\/products\/kubernetes-platform\/pricing\/.*/,/localhost/,/https\:\/\/t.me/,/docs-prv\.pcisecuritystandards\.org/,/gitlab.com\/profile/,/dash.cloudflare.com\/profile/,/example.com/,/vmware.com/,/.slack.com/,/habr.com/,/flant.ru/,/bcrypt-generator.com/,/candi\/bashible\/bashbooster/,/..\/..\/compare\//,/compare\/ru\//,/compare\/en\//,/\.yml$/,/\.yaml$/,/\.tmpl$/,/\.tpl$/,/^\/products\/kubernetes-platform\/$/,/^\/products\/virtualization-platform\/$/' \
       --swap-urls "https\:\/\/deckhouse\.io\/guides\/:/products/kubernetes-platform/guides/,https\:\/\/deckhouse\.io\/gs\/:/products/kubernetes-platform/gs/,https\:\/\/deckhouse\.io\/:/,\/documentation\/latest\/:/documentation/,\/documentation\/v1\/:/documentation/" \
       --ignore-files "404.html" --ignore-status-codes "0,429" .

if [ $? -ne 0 ] || [ $exit_code -ne 0 ]; then
  echo -e "\n\nChecking links failed!" && exit 1
fi
