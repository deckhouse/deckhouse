
version: 0.1
log:
  level: info
storage:
  s3:
    accesskey: awsaccesskey
    secretkey: awssecretkey
    region: us-west-1
    regionendpoint: http://localhost:8333
    bucket: registry
    encrypt: false
    secure: false
    v4auth: true
    chunksize: 5242880
    rootdirectory: /
    multipartcopy:
      maxconcurrency: 100
      chunksize: 33554432
      thresholdsize: 33554432
  delete:
    enabled: true
  redirect:
    disable: true
  cache:
    blobdescriptor: inmemory
http:
  addr: "{{ .hostIP }}:5001"
  #addr: 0.0.0.0:5000
  prefix: /
  secret: asecretforlocaldevelopment
  debug:
    addr: "127.0.0.1:5002"
    prometheus:
      enabled: true
      path: /metrics

{{- if eq .registry.registryMode "Proxy" }}
proxy:
  remoteurl: "{{ .registry.upstreamRegistry.upstreamRegistryScheme }}://{{ $.registry.upstreamRegistry.upstreamRegistryHost }}"
  username: "{{ .registry.upstreamRegistry.upstreamRegistryUser }}"
  password: "{{ .registry.upstreamRegistry.upstreamRegistryPassword }}"
  ttl: 72h
{{- end }}
auth:
  token:
    realm: "http://{{ .hostIP }}:5051/auth"
    service: Docker registry
    issuer: Registry server
    rootcertbundle: /system_registry_pki/token.crt
    autoredirect: false
