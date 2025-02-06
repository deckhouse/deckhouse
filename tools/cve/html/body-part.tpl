{{- if . }}
      <table>
      {{- range . }}
        <tr class="group-header"><th colspan="6">Source type: {{ .Type | toString | escapeXML }}<br>Source: {{ escapeXML .Target }}</br></th></tr>
        {{- if (eq (len .Vulnerabilities) 0) }}
        <tr><th colspan="6">No Vulnerabilities found</th></tr>
        {{- else }}
        <tr class="sub-header">
          <th>Package</th>
          <th>Vulnerability ID</th>
          <th>Severity</th>
          <th>Installed Version</th>
          <th>Fixed Version</th>
        </tr>
          {{- range .Vulnerabilities }}
        <tr class="severity-{{ escapeXML .Vulnerability.Severity }}">
          <td class="pkg-name">{{ escapeXML .PkgName }}</td>
          <td>
            <a href={{ escapeXML (index .Vulnerability.References 0) | printf "%q" }}>{{ escapeXML .VulnerabilityID }}</a>
          </td>
          <td class="severity">{{ escapeXML .Vulnerability.Severity }}</td>
          <td class="pkg-version">{{ escapeXML .InstalledVersion }}</td>
          <td>{{ escapeXML .FixedVersion }}</td>
        </tr>
          {{- end }}
        {{- end }}
        {{- if (eq (len .Misconfigurations ) 0) }}
        <tr><th colspan="6">No Misconfigurations found</th></tr>
        {{- else }}
        <tr class="sub-header">
          <th>Type</th>
          <th>Misconf ID</th>
          <th>Check</th>
          <th>Severity</th>
          <th>Message</th>
        </tr>
          {{- range .Misconfigurations }}
        <tr class="severity-{{ escapeXML .Severity }}">
          <td class="misconf-type">{{ escapeXML .Type }}</td>
          <td>{{ escapeXML .ID }}</td>
          <td class="misconf-check">{{ escapeXML .Title }}</td>
          <td class="severity">{{ escapeXML .Severity }}</td>
          <td class="link" data-more-links="off"  style="white-space:normal;"">
            {{ escapeXML .Message }}
            <br>
              <a href={{ escapeXML .PrimaryURL | printf "%q" }}>{{ escapeXML .PrimaryURL }}</a>
            </br>
          </td>
        </tr>
          {{- end }}
        {{- end }}
      {{- end }}
      </table>
{{- else }}
      <h1>Trivy Returned Empty Report</h1>
{{- end }}
