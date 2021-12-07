#!/bin/bash

# Copyright 2021 Flant JSC
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

# Use gomplate to render files in .github/workflows
# from main template .github-ci.yaml and
# partials in .github/ci_includes, .github/ci_templates

cat <<'SCRIPT_END' | docker run -i --rm \
  -v $(pwd)/../.gitlab/ci_includes:/in/gitlab_ci_includes \
  -v $(pwd)/ci_includes:/in/ci_includes \
  -v $(pwd)/ci_templates:/in/ci_templates \
  -v $(pwd)/workflow_templates:/in/workflow_templates \
  -v $(pwd)/workflows:/out/workflows \
  --entrypoint=ash \
  hairyhenderson/gomplate:v3.9.0-alpine -

# Render each file in workflow_templates
# directory and copy to /out/workflows
set -e

cat <<EOF > /in/header
#
# THIS FILE IS GENERATED, PLEASE DO NOT EDIT.
#

EOF

# Generate image_versions.yml from Gitlab configuration.
# TODO remove after full migration to Github.
(cat /in/header
 echo '{!{ define "image_versions_envs" }!}
{!{$BASE_IMAGES_REGISTRY_PATH := "registry.deckhouse.io/base_images/" }!}'
 grep '^#' /in/gitlab_ci_includes/image_versions.yml
 cat <<'EOF' | gomplate --datasource image_versions=file:///in/gitlab_ci_includes/image_versions.yml
{{- $vars := (ds "image_versions").variables -}}
{{ range $k, $v := $vars }}
{{- $k }}: "{{$v | replaceAll "${BASE_IMAGES_REGISTRY_PATH}" "{!{$BASE_IMAGES_REGISTRY_PATH}!}" }}"
{{ end -}}
EOF
echo '{!{- end -}!}'
) > /in/ci_includes/image_versions.yml

# Generate terraform_versions.yml from Gitlab configuration.
# TODO remove after full migration to Github.
(cat /in/header
 echo '{!{ define "terraform_versions_envs" }!}
# Terraform settings'
 cat <<'EOF' | gomplate --datasource terraform_versions=file:///in/gitlab_ci_includes/terraform_versions.yml
{{- $vars := (ds "terraform_versions").variables -}}
{{ range $k, $v := $vars }}
{{- $k }}: {{$v}}
{{ end -}}
EOF
echo '{!{- end -}!}'
) > /in/ci_includes/terraform_versions.yml


# Generate workflow files from workflow_templates directory.
mkdir -p /out/tmp
hasChanges=0

for f in /in/workflow_templates/* ; do
 # echo "Render $f"
  fname=$(basename $f)
  outname=${fname%.tpl}

  outarg="--out /out/tmp/${outname}"
  if echo "$fname" | grep -q '.multi.' ; then
    outarg=''
  fi

  OUTDIR=/out/tmp gomplate --left-delim '{!{' \
           --right-delim '}!}' \
           --datasource in=/in \
           --datasource actions=file:///in/ci_includes/actions_versions.yml \
           --template incl=/in/ci_includes \
           --template tpl=/in/ci_templates \
           --file $f $outarg
done


for f in /out/tmp/* ; do
  # add header and remove spaces
  cat /in/header $f | sed 's/^ *$//g' > $f.new
  mv $f.new $f

  fname=$(basename $f)
  outname=${fname%.tpl}

  if [[ -f /out/workflows/$outname ]] ; then
    if ! diff="$(diff -u /out/workflows/$outname /out/tmp/$outname)" ; then
      hasChanges=1
      echo "$diff"
    fi
  else
    hasChanges=1
    echo "New file /out/workflows/$fname"
  fi
done

if [[ $hasChanges == 1 ]] ; then
  mv /out/tmp/*.yml /out/workflows/
  echo "Render success!"
else
  echo "No changes."
fi

for file in /out/workflows/* ; do
  # Ignore md files. '== *.md' is not working in ash.
  [[ $file != ${file%.md} ]] && continue
  gomplate -i "$file"'{{ $_ := file.Read "'"$file"'" | yaml }} OK' || true
done

exit $hasChanges

SCRIPT_END
