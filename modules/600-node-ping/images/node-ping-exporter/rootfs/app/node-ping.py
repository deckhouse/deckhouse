#!/usr/bin/env python3

import subprocess
import prometheus_client
import re
import statistics
import os
import json
import glob
import better_exchook
import datetime

better_exchook.install()

FPING_CMDLINE = "/usr/sbin/fping -p 1000 -A -C 30 -B 1 -q -r 1".split(" ")
FPING_REGEX = re.compile(r"^(\S*)\s*: (.*)$", re.MULTILINE)
CONFIG_PATH = "/config/nodes.json"

registry = prometheus_client.CollectorRegistry()

prometheus_exceptions_counter = \
    prometheus_client.Counter('kube_node_ping_exceptions', 'Total number of exceptions', [], registry=registry)

prom_metrics = {"sent": prometheus_client.Counter('kube_node_ping_packets_sent_total',
                                                  'ICMP packets sent',
                                                  ['destination_node',
                                                   'destination_node_ip_address'],
                                                  registry=registry), "received": prometheus_client.Counter(
    'kube_node_ping_packets_received_total', 'ICMP packets received',
    ['destination_node', 'destination_node_ip_address'], registry=registry), "rtt": prometheus_client.Counter(
    'kube_node_ping_rtt_milliseconds_total', 'round-trip time',
    ['destination_node', 'destination_node_ip_address'], registry=registry),
                "min": prometheus_client.Gauge('kube_node_ping_rtt_min', 'minimum round-trip time',
                                               ['destination_node', 'destination_node_ip_address'],
                                               registry=registry),
                "max": prometheus_client.Gauge('kube_node_ping_rtt_max', 'maximum round-trip time',
                                               ['destination_node', 'destination_node_ip_address'],
                                               registry=registry),
                "mdev": prometheus_client.Gauge('kube_node_ping_rtt_mdev',
                                                'mean deviation of round-trip times',
                                                ['destination_node', 'destination_node_ip_address'],
                                                registry=registry)}


def validate_envs():
    envs = {"MY_NODE_NAME": os.getenv("MY_NODE_NAME"), "PROMETHEUS_TEXTFILE_DIR": os.getenv("PROMETHEUS_TEXTFILE_DIR"),
            "PROMETHEUS_TEXTFILE_PREFIX": os.getenv("PROMETHEUS_TEXTFILE_PREFIX")}

    for k, v in envs.items():
        if not v:
            raise ValueError("{} environment variable is empty".format(k))

    return envs


@prometheus_exceptions_counter.count_exceptions()
def compute_results(results):
    computed = {}

    matches = FPING_REGEX.finditer(results)
    for match in matches:
        ip = match.group(1)
        ping_results = match.group(2)
        if "duplicate" in ping_results:
            continue
        splitted = ping_results.split(" ")
        if len(splitted) != 30:
            raise ValueError("ping returned wrong number of results: \"{}\"".format(splitted))

        positive_results = [float(x) for x in splitted if x != "-"]
        if len(positive_results) > 0:
            computed[ip] = {"sent": 30, "received": len(positive_results),
                            "rtt": sum(positive_results),
                            "max": max(positive_results), "min": min(positive_results),
                            "mdev": statistics.pstdev(positive_results)}
        else:
            computed[ip] = {"sent": 30, "received": len(positive_results), "rtt": 0,
                            "max": 0, "min": 0, "mdev": 0}
    if not len(computed):
        raise ValueError("regex match\"{}\" found nothing in fping output \"{}\"".format(FPING_REGEX, results))
    return computed


@prometheus_exceptions_counter.count_exceptions()
def call_fping(ips):
    cmdline = FPING_CMDLINE + ips
    process = subprocess.run(cmdline, stdout=subprocess.PIPE,
                             stderr=subprocess.STDOUT, universal_newlines=True)
    if process.returncode == 3:
        raise ValueError("invalid arguments: {}".format(cmdline))
    if process.returncode == 4:
        raise OSError("fping reported syscall error: {}".format(process.stderr))

    return process.stdout


envs = validate_envs()

files = glob.glob(envs["PROMETHEUS_TEXTFILE_DIR"] + "*")
for f in files:
    os.remove(f)

labeled_prom_metrics = []

while True:
    with open("/config/nodes.json", "r") as f:
        config = json.loads(f.read())

    if labeled_prom_metrics:
        for node in config:
            if (node["name"], node["ipAddress"]) not in [(metric["node_name"], metric["ip"]) for metric in labeled_prom_metrics]:
                for k, v in prom_metrics.items():
                    v.remove(node["name"], node["ipAddress"])

    labeled_prom_metrics = []

    for node in config:
        metrics = {"node_name": node["name"], "ip": node["ipAddress"], "prom_metrics": {}}

        for k, v in prom_metrics.items():
            metrics["prom_metrics"][k] = v.labels(node["name"], node["ipAddress"])

        labeled_prom_metrics.append(metrics)

    out = call_fping([prom_metric["ip"] for prom_metric in labeled_prom_metrics])
    computed = compute_results(out)

    for dimension in labeled_prom_metrics:
        result = computed[dimension["ip"]]
        dimension["prom_metrics"]["sent"].inc(computed[dimension["ip"]]["sent"])
        dimension["prom_metrics"]["received"].inc(computed[dimension["ip"]]["received"])
        dimension["prom_metrics"]["rtt"].inc(computed[dimension["ip"]]["rtt"])
        dimension["prom_metrics"]["min"].set(computed[dimension["ip"]]["min"])
        dimension["prom_metrics"]["max"].set(computed[dimension["ip"]]["max"])
        dimension["prom_metrics"]["mdev"].set(computed[dimension["ip"]]["mdev"])

    prometheus_client.write_to_textfile(
        envs["PROMETHEUS_TEXTFILE_DIR"] + envs["PROMETHEUS_TEXTFILE_PREFIX"] + envs["MY_NODE_NAME"] + ".prom", registry)
