import os
import re

def parse_werf_files():
    base_dir = 'modules'
    
    for root, dirs, files in os.walk(base_dir):
        if 'images' in root.split(os.sep):
            if 'werf.inc.yaml' in files:
                module_path = os.path.dirname(root)
                rel_path = os.path.relpath(module_path, base_dir)
                with open(os.path.join(root, 'werf.inc.yaml'), 'r') as f:
                    content = f.read()
                    if 'SVACE_ENABLED' not in content:
                        # print(f"{root} - SVACE_ENABLED")
                        print(f'{root} - EMPTY')

if __name__ == '__main__':
    parse_werf_files()
