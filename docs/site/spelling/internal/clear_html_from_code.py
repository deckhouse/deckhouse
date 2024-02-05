#!/usr/bin/python3
# -*- coding: utf-8 -*-

from bs4 import BeautifulSoup
import sys


if len (sys.argv) > 1:
    html = open(sys.argv[1]).read()
    root = BeautifulSoup(html, 'html.parser')
    if root.find('code'):
        for code in root.select('code'):
            code.decompose()
    snippetcuts = root.find_all("div", {"class": "snippetcut"})
    for snippetcut in snippetcuts:
        snippetcut.decompose()
    print(root)
