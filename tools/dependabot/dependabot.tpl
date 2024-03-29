#
# THIS FILE IS GENERATED, PLEASE DO NOT EDIT.
#

version: 2

updates:
# go.mod
{{- range $_, $v := .Paths.GoMod }}
- package-ecosystem: "gomod"
  directory: "{{ $v }}"
  labels:
  - "type/dependencies"
  - "status/ok-to-test"
  schedule:
    interval: "daily"
  open-pull-requests-limit: 0
{{ end }}
# pip
{{- range $_, $v := .Paths.PIP }}
- package-ecosystem: "pip"
  directory: "{{ $v }}"
  labels:
  - "type/dependencies"
  - "status/ok-to-test"
  schedule:
    interval: "daily"
  open-pull-requests-limit: 0
{{ end }}
# npm
{{- range $_, $v := .Paths.NPM }}
- package-ecosystem: "npm"
  directory: "{{ $v }}"
  labels:
  - "type/dependencies"
  - "status/ok-to-test"
  schedule:
    interval: "daily"
  open-pull-requests-limit: 0
{{ end }}

# Do not update actions because we use templating yet dependabot tries to update autogenerated files
- package-ecosystem: "github-actions"
  directory: "/"
  labels:
  - "type/dependencies"
  - "area/dx"
  schedule:
    interval: "daily"
  open-pull-requests-limit: 0
