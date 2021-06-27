# Bash Booster 0.6 <http://www.bashbooster.net>
# =============================================
#
# Copyright (c) 2014, Dmitry Vakhrushev <self@kr41.net> and Contributors
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.
#

# normalizing, reading and evaluating key=value lines from the properties file
# regexp searches for lines with key=value, key:value, key: value etc.. pattern,
# see http://docs.oracle.com/javase/7/docs/api/java/util/Properties.html#load(java.io.Reader)
bb-ext-python 'bb-read-properties-helper' <<EOF
import re
import sys

filename = sys.argv[1]
prefix = sys.argv[2]
with open(filename, 'r') as properties:
    for line in properties:
        line = line.strip()
        match = re.match(r'^(?P<key>[^#!]*?)[\s:=]+(?P<value>.+)', line)
        if match:
            match = match.groupdict()
            match['key'] = re.sub(r'[\W]', '_', match['key'])
            print('{prefix}{key}="{value}"'.format(prefix=prefix, **match))
EOF

bb-read-properties() {
    local FILENAME="$1"
    local PREFIX="$2"

    if [[ ! -r "$FILENAME" ]]
    then
        bb-log-error "'$FILENAME' is not readable"
        return 1
    fi

    eval "$( bb-read-properties-helper "$FILENAME" "$PREFIX" )"
}


bb-ext-python 'bb-read-ini-helper' <<EOF
import re
import sys
try:
    from ConfigParser import SafeConfigParser as ConfigParser
except ImportError:
    # Python 3.x
    from configparser import ConfigParser

filename = sys.argv[1]
section = sys.argv[2]
prefix = sys.argv[3]
reader = ConfigParser()
reader.read(filename)

if not section or section == '*':
    sections = reader.sections()
else:
    sections = [section]
for section in sections:
    for key, value in reader.items(section):
        section = re.sub(r'[\W]', '_', section)
        key = re.sub(r'[\W]', '_', key)
        print(
            '{prefix}{section}_{key}="{value}"'.format(
                prefix=prefix,
                section=section,
                key=key,
                value=value
            )
        )
EOF

bb-read-ini() {
    local FILENAME="$1"
    local SECTION="$2"
    local PREFIX="$3"

    if [[ ! -r "$FILENAME" ]]
    then
        bb-log-error "'$FILENAME' is not readable"
        return 1
    fi

    eval "$( bb-read-ini-helper "$FILENAME" "$SECTION" "$PREFIX" )"
}


bb-ext-python 'bb-read-json-helper' <<EOF
import re
import sys
import json

filename = sys.argv[1]
prefix = sys.argv[2]

def serialize(value, name):
    if value is None:
        print('{0}=""'.format(name))
    elif hasattr(value, 'items'):
        for key, subvalue in value.items():
            key = re.sub(r'[\W]', '_', key)
            serialize(subvalue, name + '_' + key)
    elif hasattr(value, '__iter__'):
        print("{0}_len={1}".format(name, len(value)))
        for i, v in enumerate(value):
            serialize(v, name + '_' + str(i))
    else:
        print('{0}="{1}"'.format(name, value))

with open(filename, 'r') as json_file:
    data = json.load(json_file)
    serialize(data, prefix)

EOF

bb-read-json() {
    local FILENAME="$1"
    local PREFIX="$2"

    if [[ ! -r "$FILENAME" ]]
    then
        bb-log-error "'$FILENAME' is not readable"
        return 1
    fi

    eval "$( bb-read-json-helper "$FILENAME" "$PREFIX" )"
}


bb-ext-python 'bb-read-yaml-helper' <<EOF
import re
import sys
import yaml

filename = sys.argv[1]
prefix = sys.argv[2]

def serialize(value, name):
    if value is None:
        print('{0}=""'.format(name))
    elif hasattr(value, 'items'):
        for key, subvalue in value.items():
            key = re.sub(r'[\W]', '_', key)
            serialize(subvalue, name + '_' + key)
    elif hasattr(value, '__iter__'):
        print("{0}_len={1}".format(name, len(value)))
        for i, v in enumerate(value):
            serialize(v, name + '_' + str(i))
    else:
        print('{0}="{1}"'.format(name, value))

with open(filename, 'r') as yaml_file:
    data = yaml.load(yaml_file)
    serialize(data, prefix)

EOF

bb-ext-python 'bb-read-yaml?' <<EOF
try:
    import yaml
except ImportError:
    exit(1)

EOF

bb-read-yaml() {
    local FILENAME="$1"
    local PREFIX="$2"

    if [[ ! -r "$FILENAME" ]]
    then
        bb-log-error "'$FILENAME' is not readable"
        return 1
    fi

    eval "$( bb-read-yaml-helper "$FILENAME" "$PREFIX" )"
}


