#!/usr/bin/env python3

import json
import subprocess
import os

def oras(command_list):
    completed_process = subprocess.run(["oras"] + command_list, text=True, capture_output=True)
    completed_process.check_returncode()
    return completed_process.stdout

images_tags_path = os.getenv("IMAGES_TAGS_PATH")

with open(images_tags_path) as f:
  images = json.load(f)['Images']

registry_from = os.getenv("REGISTRY_FROM")
registry_to = os.getenv("REGISTRY_TO")

for k in images.keys():
  if k.endswith("-vex-artifact"):
    copied_image = k.removesuffix("-vex-artifact")
    sha256 = images[copied_image]['DockerImageDigest'].removeprefix('sha256:')
    from_image = f'{registry_from}:sha256-{sha256}.att'
    to_image = f'{registry_to}:sha256-{sha256}.att'
    print(f'Copying {copied_image}: {from_image} => {to_image}')
    print(oras(['cp', from_image, to_image]))
