#!/usr/bin/env python3

# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

import json
import os, sys
import socket
import ipcalc
from pyroute2 import IPRoute

class dnsmasqConfig:
  def __init__(self):
    self.interfaces = []
    self.networks = []

  def addInterface(self, interface):
    self.interfaces.append(interface)

  def addNetwork(self, newNetwork):
    self.networks.append(newNetwork)

  def render(self):
    rows = []
    rows.append('port=0')
    rows.append('')
    rows += ['interface=' + i for i in self.interfaces]
    rows.append('')

    for index, network in enumerate(self.networks):
      args = []
      netname = 'd8net' + str(index)
      args.append('set:' + netname)
      args.append(network['pool'])
      args.append(network['netmask'])
      rows.append('dhcp-range=' + ','.join(args) + ",24h")

      if 'router' in network:
        args = []
        args.append('tag:' + netname)
        args.append('option:router')
        args.append(network['router'])
        rows.append('dhcp-option=' + ','.join(args))

      if 'dns' in network:
        args = []
        args.append('tag:' + netname)
        args.append('option:dns-server')
        args.append(','.join(network['dns']))
        rows.append('dhcp-option=' + ','.join(args))

      if 'search' in network:
        args = []
        args.append('tag:' + netname)
        args.append('option:domain-search')
        args.append(','.join(network['search']))
        rows.append('dhcp-option=' + ','.join(args))

      rows.append('')

    return "\n".join(rows)

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
      if isNetworkInNetwork(network, systemNetwork):
        return ifname
  return None

def main():
  config = json.load(open("/etc/network-gateway-config/config.json", "r"))
  systemInterfaces = getSystemInterfaces()

  myDnsmasqConfig = dnsmasqConfig()

  subnetInterface = getInterfaceByNetwork(config['subnet'], systemInterfaces)
  if subnetInterface:
    myDnsmasqConfig.addInterface(subnetInterface)
  else:
    print("Error: Can't find listen interface for subnet " + config['subnet'] + ".")
    sys.exit(1)

  network = {}

  netCalc = ipcalc.Network(config['subnet'])
  network['pool']    = str(netCalc.host_first() + 49) + ',' + str(netCalc.host_last())
  network['netmask'] = str(netCalc.netmask())
  network['router']  = str(netCalc.host_first())

  if 'dns' in config:
    if 'servers' in config['dns']:
      network['dns'] = config['dns']['servers']
    if 'search' in config['dns']:
      network['search'] = config['dns']['search']

  myDnsmasqConfig.addNetwork(network)

  open('/etc/dnsmasq.conf.d/deckhouse-network-gateway.conf', 'w').write(myDnsmasqConfig.render())

if __name__ == '__main__':
  main()
