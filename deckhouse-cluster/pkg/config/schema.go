package config

// TODO: Think about storing it in yaml file instead of const
const ClusterConfigSchema = `
kind: ClusterConfiguration
apiVersions:
- apiVersion: deckhouse.io/v1alpha1
  openAPISpec:
    type: object
    additionalProperties: false
    required: [apiVersion, kind, spec, bootstrap]
    properties:
      apiVersion:
        type: string
        enum: [deckhouse.io/v1alpha1]
      kind:
        type: string
        enum: [ClusterConfiguration]
      spec:
        type: object
        additionalProperties: false
        required: [clusterType, kubernetesVersion, podSubnetCIDR, serviceSubnetCIDR]
        properties:
          clusterType:
            type: string
            enum: [Cloud, Static]
          cloud:
            type: object
            required: [provider]
            properties:
              provider:
                type: string
          podSubnetCIDR:
            type: string
          podSubnetNodeCIDRPrefix:
            type: string
            default: "24"
          serviceSubnetCIDR:
            type: string
          kubernetesVersion:
            type: string
        oneOf:
        - properties:
            clusterType:
               enum: [Handcrafted]
        - properties:
            clusterType:
               enum: [Cloud]
          cloud: {}
          required: [cloud]
      bootstrap:
        type: object
        additionalProperties: false
        required: [deckhouse, masterNodeGroup]
        properties:
          sshPublicKeys:
            type: array
            items:
              type: string
          masterNodeGroup:
            type: object
            required: [minReplicasPerZone, maxReplicasPerZone, zones]
            properties:
               minReplicasPerZone:
                 type: integer
               maxReplicasPerZone:
                 type: integer
               zones:
                 type: array
                 items:
                   type: string
          deckhouse:
            type: object
            oneOf:
            - required: [imagesRepo, devBranch, configOverrides]
            - required: [imagesRepo, releaseChannel, configOverrides]
            properties:
              imagesRepo:
                type: string
              registryDockerCfg:
                type: string
              releaseChannel:
                type: string
                enum: [Alpha, Beta, EarlyAccess, Stable, RockSolid]
              devBranch:
                type: string
              bundle:
                type: string
                enum: [Minimal, Default]
                default: Default
              logLevel:
                type: string
                enum: [Debug, Info, Error]
                default: Info
              configOverrides:
                type: object
`
