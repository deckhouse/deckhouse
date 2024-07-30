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

function tools::generate_password() {
  python3 -c 'import secrets,sys; sys.stdout.write(secrets.token_urlsafe(20))'
}

function tools::to_slug() {
  to_slug() {
    # Forcing the POSIX local so alnum is only 0-9A-Za-z
    export LANG=POSIX
    export LC_ALL=POSIX
    # Keep only alphanumeric value
    sed -e 's/[^[:alnum:]]/-/g' |
    # Keep only one dash if there is multiple one consecutively
    tr -s '-'                   |
    # Lowercase everything
    tr A-Z a-z                  |
    # Remove last dash if there is nothing after
    sed -e 's/-$//'
  }

  # Consume stdin if it exist
  if test -p /dev/stdin; then
    read -r input
  fi

  # Now check if there was input in stdin
  if test -n "${input}"; then
    echo "${input}" | to_slug
    exit
  # No stdin, let's check if there is an argument
  elif test -n "${1}"; then
    echo "${1}" | to_slug
    exit
  else
    >&2 echo "ERROR: no input found to slugify"
    return 1
  fi
}
