#!/usr/bin/env bash
{{- /*
# Copyright 2024 Flant JSC
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
*/}}

{{- $python_discovery := .Files.Get "deckhouse/candi/bashible/check_python.sh.tpl" }}
{{- tpl ( $python_discovery ) . | nindent 0 }}

cat - <<EOF | $python_binary
import pwd
import os
import re
import glob
import signal

PROC_TCP = "/proc/net/tcp"
STATE = {
        '01':'ESTABLISHED',
        '02':'SYN_SENT',
        '03':'SYN_RECV',
        '04':'FIN_WAIT1',
        '05':'FIN_WAIT2',
        '06':'TIME_WAIT',
        '07':'CLOSE',
        '08':'CLOSE_WAIT',
        '09':'LAST_ACK',
        '0A':'LISTEN',
        '0B':'CLOSING'
        }

def _load():
    with open(PROC_TCP,'r') as f:
        content = f.readlines()
        content.pop(0)
    return content

def _hex2dec(s):
    return str(int(s,16))

def _ip(s):
    ip = [(_hex2dec(s[6:8])),(_hex2dec(s[4:6])),(_hex2dec(s[2:4])),(_hex2dec(s[0:2]))]
    return '.'.join(ip)

def _remove_empty(array):
    return [x for x in array if x !='']

def _convert_ip_port(array):
    host,port = array.split(':')
    return _ip(host),_hex2dec(port)

def get_pid_for_listen_port(host, port):
    content=_load()
    for line in content:
        line_array = _remove_empty(line.split(' '))
        l_host,l_port = _convert_ip_port(line_array[1])
        if l_host != host or l_port != port:
            continue
        state = STATE[line_array[3]]
        if state != 'LISTEN':
            continue
        inode = line_array[9]
        pid = _get_pid_of_inode(inode)

        return pid
    return ''

def _get_pid_of_inode(inode):
    for item in glob.glob('/proc/[0-9]*/fd/[0-9]*'):
        try:
            if re.search(inode,os.readlink(item)):
                return item.split('/')[2]
        except:
            pass
    return ''

if __name__ == '__main__':
    pid = get_pid_for_listen_port('{{.host}}', '{{.port}}')
    print(pid)
    if pid == '':
        print('Port not fount')
        exit(0)
    try:
        os.kill(int(pid), signal.SIGKILL)
    except Exeption as e:
        print(e)

    exit(0)

EOF


