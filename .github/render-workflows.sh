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

volumesRoot=
if [[ -d workflows ]] ; then
  volumesRoot=$(pwd)
elif [[ -d .github ]] ; then
  volumesRoot=$(pwd)/.github
else
  echo "Should run from repo root directory or .github directory"
  exit 1
fi

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
  -e TARGET_UID=$(id -u) \
  -e TARGET_GID=$(id -g) \
  -e TARGET_UMASK=$(umask) \
  -e TARGET_OSTYPE=${OSTYPE} \
  -v ${volumesRoot}/ci_includes:/in/ci_includes \
  -v ${volumesRoot}/ci_templates:/in/ci_templates \
  -v ${volumesRoot}/../candi/image_versions.yml:/in/image_versions.yml \
  -v ${volumesRoot}/workflow_templates:/in/workflow_templates \
  -v ${volumesRoot}/workflows:/out/workflows \
  --entrypoint=ash \
  hairyhenderson/gomplate:v3.11.7-alpine - || dockerExit=1

# Render each file in workflow_templates
# directory and copy to /out/workflows
set -e

umask ${TARGET_UMASK}

cat <<EOF > /in/header
#
# THIS FILE IS GENERATED, PLEASE DO NOT EDIT.
#

EOF

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
           --datasource image_versions=file:///in/image_versions.yml \
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
  echo "Render success. Workflows changed."
  mv /out/tmp/*.yml /out/workflows/
  if [[ ${TARGET_OSTYPE} == linux* ]] ; then
    echo "Restore permissions to ${TARGET_UID}:${TARGET_GID}"
    chown -R ${TARGET_UID}:${TARGET_GID} /out/workflows
  fi
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
