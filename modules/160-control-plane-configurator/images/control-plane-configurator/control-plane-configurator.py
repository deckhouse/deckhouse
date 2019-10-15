#!/usr/bin/env python3

import ruamel.yaml
from ruamel.yaml.comments import CommentedMap as ordereddict  # to avoid '!!omap' in yaml

import re
import os,sys
import time, datetime
import base64
import hashlib

MANIFESTS_DIR = '/etc/kubernetes/manifests/'
USER_AUTHN_OIDC_CA_PATH = '/etc/kubernetes/oidc-ca.crt'
WEBHOOK_CONFIG_PATH = '/etc/kubernetes/authorization-webhook-config.yaml'
WEBHOOK_CONFIG_TEMPLATE = \
"""apiVersion: v1
kind: Config
clusters:
  - name: user-authz-webhook
    cluster:
      certificate-authority-data: ##CERTIFICATE_AUTHORITY_DATA##
      server: ##URL##

users:
  - name: user-authz-webhook
    user:
      client-certificate: ##CLIENT_CERTIFICATE##
      client-key: ##CLIENT_KEY##

current-context: webhook
contexts:
- context:
    cluster: user-authz-webhook
    user: user-authz-webhook
  name: webhook
"""

class manifest:
  def __init__(self, path):
    self.path = path
    self.yaml = ruamel.yaml.YAML()
    self.yaml.preserve_quotes = True
    self.data = self.yaml.load(open(self.path, 'r'))
    self.dataRefreshHash = 0

  def isApiServer(self):
    if self.data and "metadata" in self.data and "labels" in self.data["metadata"]:
      labels = self.data["metadata"]["labels"]
    else:
      return False

    return \
      ("component" in labels and labels["component"] == "kube-apiserver" and "tier" in labels and labels["tier"] == "control-plane") or \
      ("k8s-app" in labels and labels["k8s-app"] == "kube-apiserver")

  def discoverArgsPosition(self):
    for position_type in ['command', 'args']:
      for i, container in enumerate(self.data['spec']['containers']):
        if position_type in container:
          for j, arg in enumerate(container[position_type]):
            if re.match('--authorization-mode=\S+', arg):
              # all args are items
              self.argsPositionType = position_type
              self.argsPositionIsAllArgsAreSingleString = False
              self.argsPositionContainerIndex = i

            elif re.match('.*--authorization-mode=.*', arg):
              # arg in args[] and all args are in single item
              self.argsPositionType = position_type
              self.argsPositionIsAllArgsAreSingleString = True
              self.argsPositionContainerIndex = i
              self.argsPositionArgInContainerIndex = j

  def discoverProxyClientCert(self):
    cmd_summary = ""
    for container in self.data['spec']['containers']:
      if 'command' in container:
        cmd_summary += " ".join(container['command'])
      if 'args' in container:
        cmd_summary += " ".join(container['args'])
    cert_file_match = re.match('.*--proxy-client-cert-file=(\S+).*', cmd_summary)
    cert_key_match  = re.match('.*--proxy-client-key-file=(\S+).*',  cmd_summary)

    self.webhookProxyClientCertFile = cert_file_match.group(1)
    self.webhookProxyClientKeyFile  = cert_key_match.group(1)

  def loadAuthzWebhookArgs(self):
    yaml = ruamel.yaml.YAML()
    args = yaml.load(open('/etc/control-plane-configurator-authz-webhook/args.yaml', 'r'))
    self.authzWebhookCA  = args['authzWebhookCA']
    self.authzWebhookURL = args['authzWebhookURL']
    del yaml

  def loadOIDCArgs(self):
    yaml = ruamel.yaml.YAML()
    args = yaml.load(open('/etc/control-plane-configurator-oidc/args.yaml', 'r'))
    self.oidcIssuerURL = args['oidcIssuerURL']
    if 'oidcCA' in args:
      self.oidcCA = args['oidcCA']
    del yaml

  def renderWebhookConfig(self):
    global WEBHOOK_CONFIG_TEMPLATE
    tmp = WEBHOOK_CONFIG_TEMPLATE
    tmp = tmp.replace("##CLIENT_CERTIFICATE##", self.webhookProxyClientCertFile)
    tmp = tmp.replace("##CLIENT_KEY##", self.webhookProxyClientKeyFile)
    tmp = tmp.replace("##CERTIFICATE_AUTHORITY_DATA##", base64.b64encode(self.authzWebhookCA.encode('utf-8')).decode('utf-8'))
    tmp = tmp.replace("##URL##", self.authzWebhookURL)
    self.webhookConfig = tmp

  def saveWebhookConfig(self):
    global WEBHOOK_CONFIG_PATH
    if 'annotations' not in self.data['metadata']:
      self.data['metadata']['annotations'] = {}
    self.data['metadata']['annotations']['d8-control-plane-configurator-webhook-config-hash'] = hashlib.sha256(self.webhookConfig.encode('utf-8')).hexdigest()
    open(WEBHOOK_CONFIG_PATH, 'w').write(self.webhookConfig)

  def deleteWebhookConfig(self):
    global WEBHOOK_CONFIG_PATH
    if 'annotations' in self.data['metadata'] and 'd8-control-plane-configurator-webhook-config-hash' in self.data['metadata']['annotations']:
      del self.data['metadata']['annotations']['d8-control-plane-configurator-webhook-config-hash']
      if len(self.data['metadata']['annotations']) == 0:
        del self.data['metadata']['annotations']

    if os.path.isfile(WEBHOOK_CONFIG_PATH):
      os.remove(WEBHOOK_CONFIG_PATH)

  def saveOIDCCa(self):
    global USER_AUTHN_OIDC_CA_PATH
    if 'annotations' not in self.data['metadata']:
      self.data['metadata']['annotations'] = {}
    self.data['metadata']['annotations']['d8-control-plane-configurator-oidc-ca-hash'] = hashlib.sha256(self.oidcCA.encode('utf-8')).hexdigest()
    open(USER_AUTHN_OIDC_CA_PATH, 'w').write(self.oidcCA)

  def deleteOIDCCa(self):
    global USER_AUTHN_OIDC_CA_PATH
    if 'annotations' in self.data['metadata'] and 'd8-control-plane-configurator-oidc-ca-hash' in self.data['metadata']['annotations']:
      del self.data['metadata']['annotations']['d8-control-plane-configurator-oidc-ca-hash']
      if len(self.data['metadata']['annotations']) == 0:
        del self.data['metadata']['annotations']
    if os.path.isfile(USER_AUTHN_OIDC_CA_PATH):
      os.remove(USER_AUTHN_OIDC_CA_PATH)

  def configureWebhook(self):
    global WEBHOOK_CONFIG_PATH

    if self.argsPositionIsAllArgsAreSingleString:
      arg = self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType][self.argsPositionArgInContainerIndex]
      if not re.match('.*--authorization-mode=\S*Webhook.*', arg):
        arg = re.sub('--authorization-mode=(\S*)(RBAC)', r'--authorization-mode=\1Webhook,\2', arg)
      if '--authorization-webhook-config-file=' in arg:
        arg = re.sub('--authorization-webhook-config-file=\S+', r'--authorization-webhook-config-file=' + WEBHOOK_CONFIG_PATH, arg)
      else:
        arg = re.sub('(--authorization-mode=\S+)', r'\1 --authorization-webhook-config-file=' + WEBHOOK_CONFIG_PATH, arg)
      self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType][self.argsPositionArgInContainerIndex] = arg
    else:
      args = self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType]
      is_webhook_config_file_found = False
      for i, arg in enumerate(args):
        if re.match('--authorization-mode=\S+', arg) and 'Webhook' not in arg:
          args[i] = arg.replace('RBAC', 'Webhook,RBAC')

        if re.match('--authorization-webhook-config-file=\S+', arg):
          args[i] = '--authorization-webhook-config-file=' + WEBHOOK_CONFIG_PATH
          is_webhook_config_file_found = True

      if not is_webhook_config_file_found:
        args.append('--authorization-webhook-config-file=' + WEBHOOK_CONFIG_PATH)

    if "volumes" not in self.data['spec']:
      self.data['spec']['volumes'] = []
    if "volumeMounts" not in self.data['spec']['containers'][self.argsPositionContainerIndex]:
      self.data['spec']['containers'][self.argsPositionContainerIndex]['volumeMounts'] = []

    volume = ordereddict([
      ('name', 'authorization-webhook-config'),
      ('hostPath', ordereddict([
          ('path', WEBHOOK_CONFIG_PATH),
          ('type', 'FileOrCreate')
        ])
      )
    ])

    volumeMount = ordereddict([
      ('name', 'authorization-webhook-config'),
      ('mountPath', WEBHOOK_CONFIG_PATH),
      ('readOnly', True)
    ])

    if 'authorization-webhook-config' not in [v['name'] for v in self.data['spec']['volumes']]:
      self.data['spec']['volumes'].append(volume)
    if 'authorization-webhook-config' not in [vm['name'] for vm in self.data['spec']['containers'][self.argsPositionContainerIndex]['volumeMounts']]:
      self.data['spec']['containers'][self.argsPositionContainerIndex]['volumeMounts'].append(volumeMount)

  def deconfigureWebhook(self):
    global WEBHOOK_CONFIG_PATH

    if self.argsPositionIsAllArgsAreSingleString:
      arg = self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType][self.argsPositionArgInContainerIndex]
      arg = arg.replace('Webhook,RBAC', 'RBAC')
      arg = re.sub('\s?--authorization-webhook-config-file=\S+', '', arg)
      self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType][self.argsPositionArgInContainerIndex] = arg
    else:
      args = self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType]
      authorization_webhook_config_file_index = None
      for i, arg in enumerate(args):
        if re.match('--authorization-mode=\S+', arg):
          args[i] = arg.replace('Webhook,RBAC', 'RBAC')

        if re.match('--authorization-webhook-config-file=\S+', arg):
          authorization_webhook_config_file_index = i

      if authorization_webhook_config_file_index is not None:
        del args[authorization_webhook_config_file_index]

      self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType] = args

    if "volumes" in self.data['spec']:
      for i, volume in enumerate(self.data['spec']['volumes']):
        if volume['name'] == 'authorization-webhook-config':
          del self.data['spec']['volumes'][i]
          break

    if "volumeMounts" in self.data['spec']['containers'][self.argsPositionContainerIndex]:
      for i, volumeMount in enumerate(self.data['spec']['containers'][self.argsPositionContainerIndex]['volumeMounts']):
        if volumeMount['name'] == 'authorization-webhook-config':
          del self.data['spec']['containers'][self.argsPositionContainerIndex]['volumeMounts'][i]
          break

  def configureOIDC(self):
    if self.argsPositionIsAllArgsAreSingleString:
      arg = self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType][self.argsPositionArgInContainerIndex]

      if '--oidc-client-id=' in arg:
        arg = re.sub('--oidc-client-id=\S+', r'--oidc-client-id=kubernetes', arg)
      else:
        arg = arg + " --oidc-client-id=kubernetes"

      if '--oidc-groups-claim=' in arg:
        arg = re.sub('--oidc-groups-claim=\S+', r'--oidc-groups-claim=groups', arg)
      else:
        arg = arg + " --oidc-groups-claim=groups"

      if '--oidc-username-claim=' in arg:
        arg = re.sub('--oidc-username-claim=\S+', r'--oidc-username-claim=email', arg)
      else:
        arg = arg + " --oidc-username-claim=email"

      if '--oidc-issuer-url=' in arg:
        arg = re.sub('--oidc-issuer-url=\S+', r'--oidc-issuer-url=' + self.oidcIssuerURL, arg)
      else:
        arg = arg + " --oidc-issuer-url=" + self.oidcIssuerURL

      self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType][self.argsPositionArgInContainerIndex] = arg
    else:
      args = self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType]

      is_client_id_found      = False
      is_groups_claim_found   = False
      is_username_claim_found = False
      is_issuer_url_found     = False
      for i, arg in enumerate(args):
        if '--oidc-client-id=' in arg:
          args[i] = re.sub('--oidc-client-id=\S+', r'--oidc-client-id=kubernetes', arg)
          is_client_id_found = True
        if '--oidc-groups-claim=' in arg:
          args[i] = re.sub('--oidc-groups-claim=\S+', r'--oidc-groups-claim=groups', arg)
          is_groups_claim_found = True
        if '--oidc-username-claim=' in arg:
          args[i] = re.sub('--oidc-username-claim=\S+', r'--oidc-username-claim=email', arg)
          is_username_claim_found = True
        if '--oidc-issuer-url=' in arg:
          args[i] = re.sub('--oidc-issuer-url=\S+', r'--oidc-issuer-url=' + self.oidcIssuerURL, arg)
          is_issuer_url_found = True

      if not is_client_id_found:
        args.append('--oidc-client-id=kubernetes')
      if not is_groups_claim_found:
        args.append('--oidc-groups-claim=groups')
      if not is_username_claim_found:
        args.append('--oidc-username-claim=email')
      if not is_issuer_url_found:
        args.append('--oidc-issuer-url=' + self.oidcIssuerURL)

      self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType] = args

  def configureOIDCCa(self):
    global USER_AUTHN_OIDC_CA_PATH
    if self.argsPositionIsAllArgsAreSingleString:
      arg = self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType][self.argsPositionArgInContainerIndex]

      if '--oidc-ca-file=' in arg:
        arg = arg + " --oidc-ca-file=" + USER_AUTHN_OIDC_CA_PATH

      self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType][self.argsPositionArgInContainerIndex] = arg
    else:
      args = self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType]

      is_oidc_ca_file_found = False
      for i, arg in enumerate(args):
        if '--oidc-ca-file=' in arg:
          args[i] = re.sub('--oidc-ca-file=\S+', r'--oidc-ca-file=' + USER_AUTHN_OIDC_CA_PATH, arg)
          is_oidc_ca_file_found = True

      if not is_oidc_ca_file_found:
        args.append('--oidc-ca-file=' + USER_AUTHN_OIDC_CA_PATH)

      self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType] = args

    if "volumes" not in self.data['spec']:
      self.data['spec']['volumes'] = []
    if "volumeMounts" not in self.data['spec']['containers'][self.argsPositionContainerIndex]:
      self.data['spec']['containers'][self.argsPositionContainerIndex]['volumeMounts'] = []

    volume = ordereddict([
      ('name', 'oidc-ca'),
      ('hostPath', ordereddict([
          ('path', USER_AUTHN_OIDC_CA_PATH),
          ('type', 'FileOrCreate')
        ])
      )
    ])

    volumeMount = ordereddict([
      ('name', 'oidc-ca'),
      ('mountPath', USER_AUTHN_OIDC_CA_PATH),
      ('readOnly', True)
    ])

    if 'oidc-ca' not in [v['name'] for v in self.data['spec']['volumes']]:
      self.data['spec']['volumes'].append(volume)
    if 'oidc-ca' not in [vm['name'] for vm in self.data['spec']['containers'][self.argsPositionContainerIndex]['volumeMounts']]:
      self.data['spec']['containers'][self.argsPositionContainerIndex]['volumeMounts'].append(volumeMount)

  def deconfigureOIDC(self):
    if self.argsPositionIsAllArgsAreSingleString:
      arg = self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType][self.argsPositionArgInContainerIndex]
      arg = re.sub('\s?--oidc-client-id=\S+', r'', arg)
      arg = re.sub('\s?--oidc-groups-claim=\S+', r'', arg)
      arg = re.sub('\s?--oidc-username-claim=\S+', r'', arg)
      arg = re.sub('\s?--oidc-issuer-url=\S+', r'', arg)
      self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType][self.argsPositionArgInContainerIndex] = arg
    else:
      args = [
        arg for arg in self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType]
        if ('--oidc-client-id=' not in arg
        and '--oidc-groups-claim=' not in arg
        and '--oidc-username-claim=' not in arg
        and '--oidc-issuer-url=' not in arg)
      ]
      self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType] = args

  def deconfigureOIDCCa(self):
    if self.argsPositionIsAllArgsAreSingleString:
      arg = self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType][self.argsPositionArgInContainerIndex]
      arg = re.sub('\s?--oidc-ca-file=\S+', r'', arg)
      self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType][self.argsPositionArgInContainerIndex] = arg
    else:
      args = [
        arg for arg in self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType]
        if '--oidc-ca-file=' not in arg
      ]
      self.data['spec']['containers'][self.argsPositionContainerIndex][self.argsPositionType] = args

    if "volumes" in self.data['spec']:
      for i, volume in enumerate(self.data['spec']['volumes']):
        if volume['name'] == 'oidc-ca':
          del self.data['spec']['volumes'][i]
          break

    if "volumeMounts" in self.data['spec']['containers'][self.argsPositionContainerIndex]:
      for i, volumeMount in enumerate(self.data['spec']['containers'][self.argsPositionContainerIndex]['volumeMounts']):
        if volumeMount['name'] == 'oidc-ca':
          del self.data['spec']['containers'][self.argsPositionContainerIndex]['volumeMounts'][i]
          break

  def refreshData(self):
    new_data = self.yaml.load(open(self.path, 'r'))
    new_hash = hash(str(new_data))
    if new_hash != self.dataRefreshHash:
      self.data            = new_data
      self.dataRefreshHash = new_hash
      return True
    else:
      return False

  def backup(self, now):
    new_hash = hash(str(self.data))
    if self.dataRefreshHash != new_hash:
      backup_path = "/etc/kubernetes/kube-apiserver-manifest-backup-" + now.isoformat('T')
      cmd         = "cp " + self.path + " " + backup_path
      os.system(cmd)
      return backup_path
    else:
      return False

  def save(self, now):
    intermediate_hash = hash(str(self.data))
    if self.dataRefreshHash != intermediate_hash:
      if 'annotations' not in self.data['metadata']:
        self.data['metadata']['annotations'] = {}
      # every patch should be unique because of kubeadm upgrade algorithms
      self.data['metadata']['annotations']['d8-control-plane-configurator-time'] = now.isoformat('T')

      tmp_path = os.path.dirname(self.path) + "/." + os.path.basename(self.path)
      self.yaml.dump(self.data, open(tmp_path, 'w'))
      os.rename(tmp_path, self.path)

      new_hash = hash(str(self.data))
      self.dataRefreshHash = new_hash
      return True
    else:
      return False

def main():
  global WEBHOOK_CONFIG_PATH
  for filename in os.listdir(MANIFESTS_DIR):
    # Иногда etcd-манифест представлен симлинком, игнорируем
    if os.path.islink(MANIFESTS_DIR + filename):
      print("Ignoring symlink: " + MANIFESTS_DIR + filename, flush=True)
      continue
    m = manifest(MANIFESTS_DIR + filename)
    if m.isApiServer():
      print("Found kube-apiserver manifest: " + MANIFESTS_DIR + filename, flush=True)
      apiServerManifest = m
      break

  if 'apiServerManifest' in vars():
    if os.environ['IS_AUTHZ_WEBHOOK_ENABLED'] == "true":
      apiServerManifest.loadAuthzWebhookArgs()
    if os.environ['IS_OIDC_ENABLED'] == "true":
      apiServerManifest.loadOIDCArgs()

    while True:
      apiServerManifest.discoverArgsPosition()
      if apiServerManifest.refreshData():
        if os.environ['IS_AUTHZ_WEBHOOK_ENABLED'] == "true":
          print("Configuring authz webhook...", flush=True)
          apiServerManifest.discoverProxyClientCert()
          print("Discovered cert files: " + apiServerManifest.webhookProxyClientCertFile + " " + apiServerManifest.webhookProxyClientKeyFile, flush=True)
          apiServerManifest.renderWebhookConfig()
          apiServerManifest.saveWebhookConfig()
          print(WEBHOOK_CONFIG_PATH + " rendered and saved.", flush=True)
          apiServerManifest.configureWebhook()
        else:
          print("Deconfiguring authz webhook...", flush=True)
          apiServerManifest.deconfigureWebhook()
          apiServerManifest.deleteWebhookConfig()
          print(WEBHOOK_CONFIG_PATH + " wiped.", flush=True)

        if os.environ['IS_OIDC_ENABLED'] == "true":
          print("Configuring OIDC...", flush=True)
          apiServerManifest.configureOIDC()
          if hasattr(apiServerManifest, 'oidcCA'):
            apiServerManifest.saveOIDCCa()
            print("OIDC CA saved to " + USER_AUTHN_OIDC_CA_PATH)
            apiServerManifest.configureOIDCCa()
          else:
            apiServerManifest.deconfigureOIDCCa()
            apiServerManifest.deleteOIDCCa()
            print("OIDC CA " + USER_AUTHN_OIDC_CA_PATH + " wiped.")
        else:
          print("Deconfiguring OIDC...", flush=True)
          apiServerManifest.deconfigureOIDC()
          apiServerManifest.deconfigureOIDCCa()
          apiServerManifest.deleteOIDCCa()

        now = datetime.datetime.now()
        backup_path = apiServerManifest.backup(now)
        if backup_path:
          print("Manifest backed up to " + backup_path)
          apiServerManifest.save(now)
          print(apiServerManifest.path + " rendered and saved.", flush=True)
        else:
          print(apiServerManifest.path + " is up to date.")
      else:
        print("Nothing to do.", flush=True)
      time.sleep(15)
  else:
    print("ERROR: Could not find kube-apiserver manifest.", flush=True)
    sys.exit(1)

if __name__ == '__main__':
  main()
