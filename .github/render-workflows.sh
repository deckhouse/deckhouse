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

set -Eeo pipefail

# Check if actionlint is in PATH (https://github.com/rhysd/actionlint)
ACTIONLINT_RUN=no
if which actionlint 2>&1 >/dev/null ; then
  ACTIONLINT_RUN=yes
fi

if [[ $ACTIONLINT == "noop" ]] ; then
  ACTIONLINT_RUN=noop
fi

# Use gomplate to render files in .github/workflows
# from main template .github-ci.yaml and
# partials in .github/ci_includes, .github/ci_templates

dockerExit=0
cat <<'SCRIPT_END' | docker run -i --rm \
  -e ACTIONLINT_RUN=$ACTIONLINT_RUN \
  -v $(pwd)/../.gitlab/ci_includes:/in/gitlab_ci_includes \
  -v $(pwd)/ci_includes:/in/ci_includes \
  -v $(pwd)/ci_templates:/in/ci_templates \
  -v $(pwd)/workflow_templates:/in/workflow_templates \
  -v $(pwd)/workflows:/out/workflows \
  --user ${UID}:${GID} \
  --entrypoint=ash \
  hairyhenderson/gomplate:v3.10.0-alpine - || dockerExit=1

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
 grep '^#' /in/gitlab_ci_includes/image_versions.yml | grep -v Note
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
  echo "Render success. Workflows changed."
else
  echo "Render success. No changes."
fi

# Check yamls for correctness if actionlint is not available.
if [[ $ACTIONLINT_RUN != "yes" ]] ; then
  for file in /out/workflows/* ; do
    # Ignore md files. '== *.md' is not working in ash.
    [[ $file != ${file%.md} ]] && continue
    gomplate -i "$file"'{{ $_ := file.Read "'"$file"'" | yaml }} OK{{ "\n" }}' || true
  done
fi

exit $hasChanges

SCRIPT_END

# Run linter for Github Actions workflows or ask to install.
if [[ $ACTIONLINT_RUN == "no" ]] ; then
  echo
  echo 'Note: install https://github.com/rhysd/actionlint for thorough checking. ACTIONLINT=noop to mute this message.'
fi

if [[ $ACTIONLINT_RUN == "yes" ]] ; then
  echo "Run actionlint..."
  if ! actionlint ; then
    exit 1
  fi
fi

# Note: This script mimics behavior of diff utility to exit with 1
# if workflows files are changed.
# It seems a good idea in terms of automatic checking if
# render-workflows was run and properly generated workflows are committed.
exit $dockerExit
