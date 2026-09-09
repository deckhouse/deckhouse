{{- if .Values.cloudProviderAws.internal.imdsv2 }}
- name: d8.cloud-provider-aws.imdsv2
  rules:
    - alert: D8CloudProviderAWSIMDSv2NotConverged
      expr: max by (job, namespace) (cloud_data_discovery_aws_instances_imdsv1_allowed) > 0
      for: 15m
      labels:
        severity_level: "6"
        tier: cluster
        d8_module: cloud-provider-aws
      annotations:
        plk_protocol_version: "1"
        plk_markup_format: markdown
        summary: Some EC2 instances of the cluster still allow IMDSv1.
        description: |-
          `imdsv2: true` is set in `AWSClusterConfiguration`, but {{`{{ $value }}`}} EC2 instances of the cluster
          still accept metadata requests without a session token, which leaves them more exposed to unauthorized
          metadata access, including through SSRF attacks.

          Existing instances are not reconfigured by the setting alone:

          1. Master, static and bastion instances are updated by a converge run. Start the DKP installer and run `dhctl converge`.
          2. Ephemeral instances are re-created when their nodes are rolled, which happens on its own. Check that the nodes of every `CloudEphemeral` NodeGroup are being replaced: `d8 k get nodegroups`.

          The instance state is re-read once per discovery period of the cloud data discoverer (one hour by default),
          so the alert can lag behind the last node being replaced.
{{- end }}
