#!/usr/bin/env python3

# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

import json
import os, time

def main():
  config = json.load(open("/etc/network-gateway-config/config.json", "r"))

  subnet = config['subnet']
  publicAddress = config['publicAddress']
  chainName = "D8-NETWORK-GATEWAY-SNAT"
  podSubnet = os.environ['POD_SUBNET']
  serviceSubnet = os.environ['SERVICE_SUBNET']
  multicast = "224.0.0.0/4"

  cfgHash = os.environ['CONFIG_HASH']

  wipeCmd = "iptables-save -t nat | grep 'd8_netgw_cfgHash_' | grep -v " + cfgHash + " | sed 's/^-A/iptables -t nat -D/' | sh"
  os.system(wipeCmd)

# SNAT
  cmds = []
  cmds.append("iptables -w -t nat -n --list " + chainName + " >/dev/null 2>&1 || iptables -w -t nat -N " + chainName)

  rule = " -s " + subnet + " -m comment --comment d8_netgw_cfgHash_" + cfgHash + " -j " + chainName
  cmds.append("iptables -w -t nat -C POSTROUTING " + rule + " >/dev/null 2>&1 || iptables -w -t nat -I POSTROUTING 1 " + rule)

  rule = " -d " + multicast + " -m comment --comment d8_netgw_cfgHash_" + cfgHash + " -j RETURN"
  cmds.append("iptables -w -t nat -C " + chainName + rule + " >/dev/null 2>&1 || iptables -w -t nat -I " + chainName + " 1 " + rule)

  rule = " -d " + subnet + " -m comment --comment d8_netgw_cfgHash_" + cfgHash + " -j RETURN"
  cmds.append("iptables -w -t nat -C " + chainName + rule + " >/dev/null 2>&1 || iptables -w -t nat -I " + chainName + " 1 " + rule)

  rule = " -d " + podSubnet + " -m comment --comment d8_netgw_cfgHash_" + cfgHash + " -j RETURN"
  cmds.append("iptables -w -t nat -C " + chainName + rule + " >/dev/null 2>&1 || iptables -w -t nat -I " + chainName + " 1 " + rule)

  rule = " -d " + serviceSubnet + " -m comment --comment d8_netgw_cfgHash_" + cfgHash + " -j RETURN"
  cmds.append("iptables -w -t nat -C " + chainName + rule + " >/dev/null 2>&1 || iptables -w -t nat -I " + chainName + " 1 " + rule)

  rule = " -m comment --comment d8_netgw_cfgHash_" + cfgHash + " -j SNAT --to " + publicAddress
  cmds.append("iptables -w -t nat -C " + chainName + rule + " >/dev/null 2>&1 || iptables -w -t nat -A " + chainName + rule)

# FORWARD
  rule = " -s " + subnet + " -j ACCEPT"
  cmds.append("iptables -w -t filter -C FORWARD" + rule + " >/dev/null 2>&1 || iptables -w -t filter -I FORWARD" + " 1 " + rule)

  rule = " -d " + subnet + " -j ACCEPT"
  cmds.append("iptables -w -t filter -C FORWARD" + rule + " >/dev/null 2>&1 || iptables -w -t filter -I FORWARD" + " 1 " + rule)

  while True:
    os.system(';'.join(cmds))
    time.sleep(60)

if __name__ == '__main__':
  main()
