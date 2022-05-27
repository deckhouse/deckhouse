#!/usr/bin/env python3

# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

import json
import os, sys
import socket
from pyroute2 import IPRoute

KEEPALIVED_CONFIG_TEMPLATE = """
global_defs {
  preempt_delay 300
  advert_int 1
  vrrp_garp_master_refresh 30
}

%(vrrp_instances)s
"""

VRRP_INSTANCE_TEMPLATE = """
vrrp_instance %(name)s {
  state %(state)s
  interface %(interface)s
  virtual_router_id %(virtual_router_id)s
  priority %(priority)s
  %(preempt)s
  authentication {
    auth_type PASS
    auth_pass %(auth_pass)s
  }
  virtual_ipaddress {
%(virtual_ipaddresses)s
  }
}
"""

POD_NUMBER = int(os.environ['POD_NAME'].split('-')[-1])

def isNetworkInNetwork(net1, net2):
  net1Address, net1Prefix = net1.split('/')
  net2Address, net2Prefix = net2.split('/')

  a, b, c, d = net1Address.split('.')
  net1Decimal = int(a) * 256 ** 3 + int(b) * 256 ** 2 + int(c) * 256 + int(d)

  a, b, c, d = net2Address.split('.')
  net2Decimal = int(a) * 256 ** 3 + int(b) * 256 ** 2 + int(c) * 256 + int(d)

  net1Mask=(0xFFFFFFFF << (32 - int(net1Prefix))) & 0xFFFFFFFF
  net2Mask=(0xFFFFFFFF << (32 - int(net2Prefix))) & 0xFFFFFFFF
  return (net1Decimal & net1Mask & net2Mask) == (net2Decimal & net2Mask)

def getSystemInterfaces():
  ipr = IPRoute()
  ifaces = {}
  for i in ipr.get_addr(family=socket.AF_INET):
    attrs = dict(i['attrs'])
    if attrs['IFA_LABEL'] not in ifaces:
      ifaces[attrs['IFA_LABEL']] = []
    ifaces[attrs['IFA_LABEL']].append(attrs['IFA_ADDRESS'] + '/' + str(i['prefixlen']))
  return ifaces

def getInterfaceByNetwork( network, systemInterfaces ):
  for ifname, systemNetworks in systemInterfaces.items():
    for systemNetwork in systemNetworks:
      if isNetworkInNetwork(systemNetwork, network):
        return ifname
  return None

def getInterfaceByDefaultRoute():
  ipr = IPRoute()
  defaultRouteAttrs = dict(ipr.get_default_routes(family=socket.AF_INET, table=254)[0]['attrs'])
  link = ipr.get_links(defaultRouteAttrs['RTA_OIF'])
  linkAttrs = dict(link[0]['attrs'])
  return linkAttrs['IFLA_IFNAME']

def main():
  global POD_NUMBER, VRRP_INSTANCE_TEMPLATE, KEEPALIVED_CONFIG_TEMPLATE

  config = json.load(open("/etc/keepalived-instance-config/config.json", "r"))
  authPass = open("/etc/keepalived-instance-secret/authPass", "r").read()
  systemInterfaces = getSystemInterfaces()

  vrrpInstanceRenderedConfigs = []
  for vrrpInstanceIndex, vrrpInstance in enumerate(config['vrrpInstances']):
    vrrpInstanceArgs = {}
    vrrpInstanceArgs['name'] = "instance_" + str(vrrpInstance['id'])
    vrrpInstanceArgs['virtual_router_id'] = vrrpInstance['id']
    vrrpInstanceArgs['priority'] = (vrrpInstanceIndex + POD_NUMBER) % config['replicas'] + 1
    vrrpInstanceArgs['state'] = 'BACKUP'
    vrrpInstanceArgs['preempt'] = "nopreempt" if 'preempt' in vrrpInstanceArgs and not vrrpInstanceArgs['preempt'] else "preempt"
    vrrpInstanceArgs['auth_pass'] = authPass

    if vrrpInstance['interface']['detectionStrategy'] == "Name":
      vrrpInstanceArgs['interface'] = vrrpInstance['interface']['name']
    elif vrrpInstance['interface']['detectionStrategy'] == "NetworkAddress":
      vrrpInstanceArgs['interface'] = getInterfaceByNetwork(vrrpInstance['interface']['networkAddress'], systemInterfaces)
    elif vrrpInstance['interface']['detectionStrategy'] == "DefaultRoute":
      vrrpInstanceArgs['interface'] = getInterfaceByDefaultRoute()

    if not vrrpInstanceArgs['interface']:
      print("Error: could not find interface for VRRP instance with ID '" + str(vrrpInstance['id']) + "'")
      sys.exit(1)

    vipInterfacesStrings = []
    for vip in vrrpInstance['virtualIPAddresses']:
      if 'interface' in vip:
        if vip['interface']['detectionStrategy'] == "Name":
         iface = vip['interface']['name']
        elif vip['interface']['detectionStrategy'] == "NetworkAddress":
          iface = getInterfaceByNetwork(vip['interface']['networkAddress'], systemInterfaces)
        elif vip['interface']['detectionStrategy'] == "DefaultRoute":
          iface = getInterfaceByDefaultRoute()
        if not iface:
          print("Error: could not find interface for virtual IP '" + vip['address'] + "' of VRRP instance with ID '" + str(vrrpInstance['id']) + "'")
          sys.exit(1)
      else:
        iface = vrrpInstanceArgs['interface']

      vipInterfacesStrings.append("    " + vip['address'] + " dev " + iface)

    vrrpInstanceArgs['virtual_ipaddresses'] = "\n".join(vipInterfacesStrings)
    vrrpInstanceRenderedConfigs.append(VRRP_INSTANCE_TEMPLATE % vrrpInstanceArgs)

  keepalivedConfigArgs = {}
  keepalivedConfigArgs['vrrp_instances'] = "\n".join(vrrpInstanceRenderedConfigs)

  keepalivedRenderedConfig = KEEPALIVED_CONFIG_TEMPLATE % keepalivedConfigArgs

  open('/etc/keepalived/keepalived.conf', 'w').write(keepalivedRenderedConfig)

if __name__ == '__main__':
  main()
