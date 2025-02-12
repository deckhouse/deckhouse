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

import os
import argparse


parser = argparse.ArgumentParser()
parser.add_argument("-t", "--token", type=str, help="Loop token")
parser.add_argument("-i", "--channel-id", type=str, help="Loop channel id to sen notification to")
parser.add_argument("-m", "--message", type=str, help="Mesage to send")
args = parser.parse_args()