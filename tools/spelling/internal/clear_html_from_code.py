#!/usr/bin/python3
# -*- coding: utf-8 -*-

from bs4 import BeautifulSoup
import sys


if len (sys.argv) > 1:
    html = open(sys.argv[1]).read()
    root = BeautifulSoup(html, 'html.parser')
    if root.find("img", {"class": "oss__item-logo"}):
        for logo in root.select("img", {"class": "oss__item-logo"}):
            logo.decompose()
    if root.find('code'):
        for code in root.select('code'):
            code.decompose()
    snippetcuts = root.find_all("div", {"class": "snippetcut"})
    for snippetcut in snippetcuts:
        snippetcut.decompose()
    if root.find("div", {"class": "tabs"}):
        tabs = root.find("div", {"class": "tabs"})
        tabs.decompose()
    if root.find("ul", {"class": "resources"}):
        for ul in root.select("ul", {"class": "resources"}):
            spans = root.find_all('span')
            for spn in spans:
                spn.decompose()
    if root.find('table'):
        tables = root.find_all('table')
        for table in tables:
            ths = table.find_all('th')
            for th in ths:
                print(th.get_text())
            tds = table.find_all('td')
            for td in tds:
                print(td.get_text())
            table.decompose()
    print(root)
