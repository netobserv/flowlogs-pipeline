#!/usr/bin/env bash

set -eou pipefail

md_file="./docs/api.md"

grep "placeholder @" "$md_file" | while read line; do
    type=$(echo "$line" | sed -r 's/^placeholder @(.*):(.*)@/\1/')
    indent=$(echo "$line" | sed -r 's/^placeholder @(.*):(.*)@/\2/')
    repl=$(go doc -all -short -C pkg/api $type | sed -r ':x ; /\/\// { N ; s/\/\/ (.*)\n/\1##/ ; bx }' | grep "##" | sed -r "s/\s*(.*)##.*\"(.*)\"/$(printf "%${indent}s")\2: \1/")
    awk -v inject="${repl}" "/placeholder @$type:$indent@/{print inject;next}1" $md_file > "$md_file.tmp"
done

rm $md_file
mv "$md_file.tmp" $md_file
