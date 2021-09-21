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

sed -i 's/\# \"\\e\[5~\": history-search-backward/\"\\e\[5~\": history-search-backward/' /etc/inputrc
sed -i 's/^\# \"\\e\[6~\": history-search-forward/\"\\e\[6~\": history-search-forward/' /etc/inputrc

sed -i 's/\#force_color_prompt=yes/force_color_prompt=yes/' /root/.bashrc
sed -i 's/01;32m/01;31m/' /root/.bashrc

mkdir -p /etc/bash_completion.d
kubectl completion bash >/etc/bash_completion.d/kubectl

completion="if [ -f /etc/bash_completion ] && ! shopt -oq posix; then . /etc/bash_completion ; fi"
grep -qF -- "$completion" /root/.bashrc || echo "$completion" >> /root/.bashrc
