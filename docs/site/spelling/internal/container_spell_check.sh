#!/bin/bash

set -e

arg_site_lang="${1:?ERROR: Site language \'en\' or \'ru\' should be specified as the first argument.}"
str=$'\n'
ex_result=0

if [[ "$arg_site_lang" == "en" ]]; then
  language="en_US,dev_OPS"
  indicator="EN"
elif [[ "$arg_site_lang" == "ru" ]]; then
  language="ru_RU,en_US,dev_OPS"
  indicator="RU"
fi

if [ -n "$2" ]; then
  arg_target_page=$2
fi

cp /usr/share/hunspell/en_US.aff  /usr/share/hunspell/en_US.aff.orig
cp /usr/share/hunspell/en_US.dic  /usr/share/hunspell/en_US.dic.orig
iconv --from ISO8859-1 /usr/share/hunspell/en_US.aff.orig > /usr/share/hunspell/en_US.aff
iconv --from ISO8859-1 /usr/share/hunspell/en_US.dic.orig > /usr/share/hunspell/en_US.dic
rm /usr/share/hunspell/en_US.aff.orig
rm /usr/share/hunspell/en_US.dic.orig
sed -i 's/SET ISO8859-1/SET UTF-8/' /usr/share/hunspell/en_US.aff

echo "Checking $arg_site_lang docs..."

if [ -n "$2" ]; then
  if [ -n "$3" ]; then
    python3 clear_html_from_code.py $arg_target_page | sed '/<!-- spell-check-ignore -->/,/<!-- end-spell-check-ignore -->/d' | html2text -utf8 | sed '/^$/d'
  else
    check=1
    if test -f "filesignore"; then
      while read y; do
        if [[ "$arg_target_page" =~ "$y" ]]; then
          unset check
          check=0
        fi
      done <<-__EOF__
  $(cat ./filesignore)
__EOF__
      if [ "$check" -eq 1 ]; then
        echo "Checking $arg_target_page..."
        result=$(python3 clear_html_from_code.py $arg_target_page | sed '/<!-- spell-check-ignore -->/,/<!-- end-spell-check-ignore -->/d' | html2text -utf8 | sed '/^$/d' | hunspell -d $language -l)
        if [ -n "$result" ]; then
          echo $result | sed 's/\s\+/\n/g'
        fi
      else
        echo "Ignoring $arg_target_page..."
      fi
    fi
  fi
else
  for file in `find ./ -type f -name "*.html"`
  do
    check=1
    if test -f "filesignore"; then
      while read y; do
        if [[ "$file" =~ "$y" ]]; then
          unset check
          check=0
        fi
      done <<-__EOF__
  $(cat ./filesignore)
__EOF__
      if [ "$check" -eq 1 ]; then
        result=$(python3 clear_html_from_code.py $file | sed '/<!-- spell-check-ignore -->/,/<!-- end-spell-check-ignore -->/d' | html2text -utf8 | sed '/^$/d' | hunspell -d $language -l)
        if [ -n "$result" ]; then
          unset ex_result
          ex_result=1
          echo $str
          echo "$indicator: checking $file..."
          echo $result | sed 's/\s\+/\n/g'
        fi
      else
        echo "Ignoring $indicator: $file..."
      fi
    fi
  done
  if [ "$ex_result" -eq 1 ]; then
    exit 1
  fi
fi
