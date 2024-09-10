{{- if . }}
      {{- range . }}
Source type: `{{ escapeXML .Type }}`
Source: `{{ escapeXML .Target }}`
        {{- if (eq (len .Vulnerabilities) 0) }}

*No Vulnerabilities found*
        {{- else }}

          | Package | Vulnerability ID | Severity | Installed Version | Fixed Version |
          | :------ | :--------------- | :------- | :---------------- | :------------ |
          {{- range .Vulnerabilities }}
          | {{ escapeXML .PkgName }} | [{{ escapeXML .VulnerabilityID }}]({{ escapeXML (index .Vulnerability.References 0) | printf "%s" }}) | {{ escapeXML .Vulnerability.Severity }} | {{ escapeXML .InstalledVersion }} | {{ escapeXML .FixedVersion }} |
          {{- end }}
        {{- end }}
        {{- if (eq (len .Misconfigurations ) 0) }}

*No Misconfigurations found*
        {{- else }}

          | Type | Misconf ID | Check | Severity | Message |
          |:-----|:-----------|:------|:---------|:--------|
          {{- range .Misconfigurations }}
          | {{ escapeXML .Type }} | {{ escapeXML .ID }} | {{ escapeXML .Title }} | {{ escapeXML .Severity }} | [{{ escapeXML .Message }}]({{ escapeXML .PrimaryURL | printf "%s" }}) |
          {{- end }}
        {{- end }}
      {{- end }}
{{- else }}
*Trivy Returned Empty Report*
{{- end }}
