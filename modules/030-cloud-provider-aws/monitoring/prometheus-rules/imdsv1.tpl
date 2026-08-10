{{- if not .Values.cloudProviderAws.internal.imdsv2 }}
- name: d8.cloud-provider-aws.imdsv1
  rules:
    - alert: D8CloudProviderAWSIMDSv1Enabled
      expr: vector(1)
      for: 5m
      labels:
        severity_level: "9"
        tier: cluster
        d8_module: cloud-provider-aws
      annotations:
        plk_protocol_version: "1"
        plk_markup_format: markdown
        summary: The AWS cluster configuration allows EC2 instances to use IMDSv1.
        description: |-
          The AWS cluster configuration allows EC2 instances to use IMDSv1, which does not require session tokens
          and is more vulnerable to unauthorized metadata access, including through SSRF attacks.

          To disable IMDSv1:

          1. Run `d8 system edit provider-cluster-configuration`.
          2. Set `provider.imdsv2: true` in `AWSClusterConfiguration`.
          3. Run `dhctl converge` to apply the change to the cluster infrastructure.

          Before enabling IMDSv2, make sure that all applications accessing EC2 instance metadata support IMDSv2.
{{- end }}
