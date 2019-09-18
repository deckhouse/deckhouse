#!/usr/bin/env python3

from http.server import BaseHTTPRequestHandler,HTTPServer
from socketserver import ThreadingMixIn
import urllib.request, urllib.parse, ssl
import json
import base64
import re
import os

PORT_NUMBER = 8000
PROMETHEUS_URL = os.environ['PROMETHEUS_URL']

context = ssl.create_default_context()
context.load_cert_chain('/etc/ssl/prometheus-api-client-tls/tls.crt', '/etc/ssl/prometheus-api-client-tls/tls.key')
context.check_hostname = False
context.verify_mode = ssl.CERT_NONE

class metricHandlerNothingToDo(Exception):
  pass

class metricHandler:
  def __init__(self, args):
    self.object_type = args[0]
    self.metric_name = args[1]
    self.selector    = args[2]
    self.group_by    = args[3]

    namespace_matches = re.match('.*namespace="([0-9a-zA-Z_\-]+)".*', self.selector)
    if namespace_matches:
      self.namespace = namespace_matches[1]
    else:
      if re.match('.*namespace=~.*', self.selector):
        raise Exception("Multiple namespaces are not implemented. Selector: " + self.selector)
      else:
        raise Exception("No 'namespace=' label in selector '"+self.selector+"' given.")

    config = json.load(open("/etc/prometheus-reverse-proxy/reverse-proxy.json","r"))
    if self.object_type in config and self.metric_name in config[self.object_type]:
      self.metric_config = config[self.object_type][self.metric_name]
    else:
      # metric "aaa" for object "bbb" not configured
      raise metricHandlerNothingToDo("Metric '"+self.metric_name+"' for object '" + self.object-name + "' not configured.")

    if "namespaced" in self.metric_config and self.namespace in self.metric_config["namespaced"]:
      self.query_template = self.metric_config["namespaced"][self.namespace]
    elif "cluster" in self.metric_config:
      self.query_template = self.metric_config["cluster"]
    else:
      raise metricHandlerNothingToDo("Metric '"+self.metric_name+"' for object '" + self.object-name + "' not configured for namespace '"+self.namespace+"' or cluster-wide.")

  def render_query(self):
    query = self.query_template.replace("<<.LabelMatchers>>", self.selector)
    query = query.replace("<<.GroupBy>>", self.group_by)
    return query

# custom_namespaced_pod_metric
class myHandler(BaseHTTPRequestHandler):
  def _handle_custom_query(self):
    global context
    url_args = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
    custom_metric_args = url_args['query'][0].split('::')[1:]

    try:
      metric_handler = metricHandler(custom_metric_args)
    except metricHandlerNothingToDo as e:
      print("Warning: " + str(e), flush=True)
      self.send_response(200)
      self.send_header('Content-type', 'application/json')
      self.wfile.write(b'{"status":"success","data":{"resultType":"vector","result":[]}}')
      return
    except Exception as e:
      print("Error: " + str(e), flush=True)
      self.send_response(500)
      self.end_headers()
      self.wfile.write(("Error: " + str(e)).encode("utf-8"))
      return

    query = metric_handler.render_query()
    url_args["query"] = [query]

    resp = urllib.request.urlopen(PROMETHEUS_URL + "/main/api/v1/query?" + urllib.parse.urlencode(url_args,doseq=True), context=context)
    self.send_response(resp.code)
    self.send_header('Content-type',resp.info()["content-type"])
    self.end_headers()
    self.wfile.write(resp.read())

  def _proxy_pass(self):
    global context
    resp = urllib.request.urlopen(PROMETHEUS_URL + self.path, context=context)
    self.send_response(resp.code)
    self.send_header('Content-type',resp.info()["content-type"])
    self.end_headers()
    self.wfile.write(resp.read())

  def _healthz(self):
    self.send_response(200)
    self.end_headers()
    self.wfile.write(b"Ok.\n")

  def do_GET(self):
    if self.path.startswith('/main/api/v1/query?query=custom_metric%3A%3A'):
      self._handle_custom_query()
    elif self.path.startswith('/healthz'):
      self._healthz()
    else:
      self._proxy_pass()
    return

class ThreadedHTTPServer(ThreadingMixIn, HTTPServer):
    """Handle requests in a separate thread."""

try:
  server = ThreadedHTTPServer(('', PORT_NUMBER), myHandler)
  print('Started httpserver on port ' + str(PORT_NUMBER), flush=True)

  #Wait forever for incoming http requests
  server.serve_forever()

except KeyboardInterrupt:
  print('^C received, shutting down the web server')
  server.socket.close()
