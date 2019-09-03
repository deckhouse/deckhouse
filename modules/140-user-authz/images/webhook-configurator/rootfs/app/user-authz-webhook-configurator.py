#!/usr/bin/env python3

import ruamel.yaml
import re
import os,sys
import time, datetime
import base64

ACTION = os.environ['ACTION']
MANIFESTS_DIR = '/etc/kubernetes/manifests/'
WEBHOOK_CONFIG_PATH = '/etc/kubernetes/authorization-webhook-config.yaml'
WEBHOOK_CONFIG_TEMPLATE = \
"""apiVersion: v1
kind: Config
clusters:
  - name: user-authz-webhook
    cluster:
      certificate-authority-data: ##CERTIFICATE_AUTHORITY_DATA##
      server: https://127.0.0.1:40443/

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

  def loadWebhookCertificateAuthorityData(self):
    self.webhookCertificateAuthorityData = open('/etc/ssl/user-authz-webhook/ca.crt', 'r').read()

  def renderWebhookConfig(self):
    global WEBHOOK_CONFIG_TEMPLATE
    tmp = WEBHOOK_CONFIG_TEMPLATE
    tmp = tmp.replace("##CLIENT_CERTIFICATE##", self.webhookProxyClientCertFile)
    tmp = tmp.replace("##CLIENT_KEY##", self.webhookProxyClientKeyFile)
    tmp = tmp.replace("##CERTIFICATE_AUTHORITY_DATA##", base64.b64encode(self.webhookCertificateAuthorityData.encode('utf-8')).decode('utf-8'))
    self.webhookConfig = tmp

  def saveWebhookConfig(self):
    global WEBHOOK_CONFIG_PATH
    open(WEBHOOK_CONFIG_PATH, 'w').write(self.webhookConfig)

  def deleteWebhookConfig(self):
    global WEBHOOK_CONFIG_PATH
    if os.path.isfile(WEBHOOK_CONFIG_PATH):
      os.remove(WEBHOOK_CONFIG_PATH)

  def configureWebhook(self):
    global WEBHOOK_CONFIG_PATH

    for position_type in ['command', 'args']:
      for i, container in enumerate(self.data['spec']['containers']):
        if position_type in container:
          for j, arg in enumerate(container[position_type]):
            if re.match('--authorization-mode=\S+', arg):
              # all args are items
              if "Webhook" not in arg:
                self.data['spec']['containers'][i][position_type][j] = arg.replace('RBAC', 'Webhook,RBAC')

              is_webhook_config_file_found = False
              for k, arg_k in enumerate(container[position_type]):
                if re.match('--authorization-webhook-config-file=\S+', arg_k):
                  self.data['spec']['containers'][i][position_type][k] = '--authorization-webhook-config-file=' + WEBHOOK_CONFIG_PATH
                  is_webhook_config_file_found = True

              if not is_webhook_config_file_found:
                self.data['spec']['containers'][i][position_type].append('--authorization-webhook-config-file=' + WEBHOOK_CONFIG_PATH)

              target_container_index = i

            elif re.match('.*--authorization-mode=.*', arg):
              # arg in args[] and all args are in single item
              tmp = arg
              if not re.match('.*--authorization-mode=\S*Webhook.*', arg):
                tmp = re.sub('--authorization-mode=(\S*)(RBAC)', r'--authorization-mode=\1Webhook,\2', tmp)
              if '--authorization-webhook-config-file=' in arg:
                tmp = re.sub('--authorization-webhook-config-file=\S+', r'--authorization-webhook-config-file=' + WEBHOOK_CONFIG_PATH, tmp)
              else:
                tmp = re.sub('(--authorization-mode=\S+)', r'\1 --authorization-webhook-config-file=' + WEBHOOK_CONFIG_PATH, tmp)
              self.data['spec']['containers'][i][position_type][j] = tmp

              target_container_index = i

    if "volumes" not in self.data['spec']:
      self.data['spec']['volumes'] = []
    if "volumeMounts" not in self.data['spec']['containers'][target_container_index]:
      self.data['spec']['containers'][target_container_index]['volumeMounts'] = []

    volume = {'name': 'authorization-webhook-config', 'hostPath': {'path': WEBHOOK_CONFIG_PATH, 'type': 'FileOrCreate'}}
    volumeMount = {'name': 'authorization-webhook-config', 'mountPath': WEBHOOK_CONFIG_PATH, 'readOnly': True}
    if 'authorization-webhook-config' not in [volume['name'] for volume in self.data['spec']['volumes']]:
      self.data['spec']['volumes'].append(volume)
    if 'authorization-webhook-config' not in [volumeMount['name'] for volumeMount in self.data['spec']['containers'][target_container_index]['volumeMounts']]:
      self.data['spec']['containers'][target_container_index]['volumeMounts'].append(volumeMount)

  def deconfigureWebhook(self):
    global WEBHOOK_CONFIG_PATH
    for position_type in ['command', 'args']:
      for i, container in enumerate(self.data['spec']['containers']):
        if position_type in container:
          for j, arg in enumerate(container[position_type]):
            if re.match('--authorization-mode=\S+', arg):
              # arg in args[] and all args are items
              self.data['spec']['containers'][i][position_type][j] = arg.replace('Webhook,RBAC', 'RBAC')

              for k, arg_k in enumerate(container[position_type]):
                if re.match('--authorization-webhook-config-file=\S+', arg_k):
                  del self.data['spec']['containers'][i][position_type][k]

              target_container_index = i

            elif re.match('.*--authorization-mode=.*', arg):
              # arg in args[] and all args are in single item
              tmp = arg.replace('Webhook,RBAC', 'RBAC')
              tmp = re.sub('--authorization-webhook-config-file=\S+', '', tmp)
              self.data['spec']['containers'][i][position_type][j] = tmp

              target_container_index = i

    if "volumes" in self.data['spec']:
      for i, volume in enumerate(self.data['spec']['volumes']):
        if volume['name'] == 'authorization-webhook-config':
          del self.data['spec']['volumes'][i]

    if "volumeMounts" in self.data['spec']['containers'][target_container_index]:
      for i, volumeMount in enumerate(self.data['spec']['containers'][target_container_index]['volumeMounts']):
        if volumeMount['name'] == 'authorization-webhook-config':
          del self.data['spec']['containers'][target_container_index]['volumeMounts'][i]

  def refreshData(self):
    new_data = self.yaml.load(open(self.path, 'r'))
    new_hash = hash(str(new_data))
    if new_hash != self.dataRefreshHash:
      self.data            = new_data
      self.dataRefreshHash = new_hash
      return True
    else:
      return False

  def backup(self):
    new_hash = hash(str(self.data))
    if self.dataRefreshHash != new_hash:
      now         = datetime.datetime.now()
      backup_path = "/etc/kubernetes/kube-apiserver-manifest-backup-" + now.strftime("%Y-%m-%d-%H-%M-%S-%f")
      cmd         = "cp " + self.path + " " + backup_path
      os.system(cmd)
      return backup_path
    else:
      return False

  def save(self):
    new_hash = hash(str(self.data))
    if self.dataRefreshHash != new_hash:
      tmp_path = os.path.dirname(self.path) + "/." + os.path.basename(self.path)
      self.yaml.dump(self.data, open(tmp_path, 'w'))
      os.rename(tmp_path, self.path)
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
    while True:
      if apiServerManifest.refreshData():
        if ACTION == "configure":
          print("Configuring...", flush=True)
          apiServerManifest.discoverProxyClientCert()
          print("Discovered cert files: " + apiServerManifest.webhookProxyClientCertFile + " " + apiServerManifest.webhookProxyClientKeyFile, flush=True)
          apiServerManifest.loadWebhookCertificateAuthorityData()
          apiServerManifest.renderWebhookConfig()
          apiServerManifest.saveWebhookConfig()
          print(WEBHOOK_CONFIG_PATH + " rendered and saved.", flush=True)
          apiServerManifest.configureWebhook()
          backup_path = apiServerManifest.backup()
          if backup_path:
            print("Manifest backed up to " + backup_path)
            apiServerManifest.save()
            print(apiServerManifest.path + " rendered and saved.", flush=True)
          else:
            print(apiServerManifest.path + " is already configured.")
        elif ACTION == "deconfigure":
          print("Deconfiguring.", flush=True)
          apiServerManifest.deconfigureWebhook()
          backup_path = apiServerManifest.backup()
          if backup_path:
            print("Manifest backed up to " + backup_path)
            apiServerManifest.save()
            print(apiServerManifest.path + " rendered and saved.", flush=True)
          else:
            print(apiServerManifest.path + " is already deconfigured.")
          apiServerManifest.deleteWebhookConfig()
          print(WEBHOOK_CONFIG_PATH + " wiped.", flush=True)
        else:
          print("Nothing to do.", flush=True)
      else:
        print("Nothing to do.", flush=True)
      time.sleep(15)
  else:
    print("ERROR: Could not find kube-apiserver manifest.", flush=True)
    sys.exit(1)

if __name__ == '__main__':
  main()
