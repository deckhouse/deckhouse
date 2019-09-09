#!/usr/bin/env python3

from http.server import BaseHTTPRequestHandler,HTTPServer
import json
import re
import os

LISTEN_PORT = 4080
LISTEN_ADDRESS = '127.0.0.1'
CONFIG_PATH = "/etc/user-authz-webhook/config.json"
SYSTEM_NAMESPACES = ["antiopa", "kube-.*", "d8-.*", "loghouse", "default"]

applied_config_mtime = 0
dictionary = {}

class myHandler(BaseHTTPRequestHandler):
  def log_message(self, format, *args):
    return

  def _healthz(self):
    self.send_response(200)
    self.end_headers()
    self.wfile.write(b"Ok.\n")

  def _404(self):
    self.send_response(404)
    self.end_headers()
    self.wfile.write(b"Wrong way.\n")

  def _check_user_access(self):
    content_length = int(self.headers['Content-Length'])
    post_data      = self.rfile.read(content_length)
    request        = json.loads(post_data)

    global applied_config_mtime
    global directory

    current_config_mtime = os.path.getmtime(CONFIG_PATH)
    if applied_config_mtime != current_config_mtime:
      applied_config_mtime = current_config_mtime
      config = json.load(open(CONFIG_PATH, 'r'))

      directory = {"User": {}, "Group": {}, "ServiceAccount": {}}
      for crd in config["crds"]:
        for subject in crd["spec"]["subjects"]:
          kind = subject["kind"]
          name = "system:serviceaccount:" + subject["namespace"] + ":" + subject["name"] if kind == "ServiceAccount" else subject["name"]

          if name not in directory[kind]:
            directory[kind][name] = []
          if "limitNamespaces" in crd["spec"]:
            directory[kind][name] += crd["spec"]["limitNamespaces"]
          if "allowAccessToSystemNamespaces" in crd["spec"] and crd["spec"]["allowAccessToSystemNamespaces"]:
            directory[kind][name] += SYSTEM_NAMESPACES

    is_our_guy = False
    allowed_namespaces = []
    if "user" in request["spec"] and request["spec"]["user"] in directory["User"]:
      is_our_guy = True
      allowed_namespaces += directory["User"][request["spec"]["user"]]

    if "user" in request["spec"] and request["spec"]["user"] in directory["ServiceAccount"]:
      is_our_guy = True
      allowed_namespaces += directory["ServiceAccount"][request["spec"]["user"]]

    if "group" in request["spec"]:
      for group in request["spec"]["group"]:
        if group in directory["Group"]:
          is_our_guy = True
          allowed_namespaces += directory["Group"][group]

    if is_our_guy and "resourceAttributes" in request["spec"] and "namespace" in request["spec"]["resourceAttributes"]:
      request["status"]={"allowed": False, "denied": True}
      request_namespace = request["spec"]["resourceAttributes"]["namespace"]
      for pattern in set(allowed_namespaces):
        if re.match("^" + pattern + "$", request_namespace):
          request["status"]={"allowed": False}
          break
    else:
      request["status"]={"allowed": False}

    self.send_response(200)
    self.end_headers()
    self.wfile.write(json.dumps(request).encode("utf-8"))

  def do_GET(self):
    if self.path.startswith('/healthz'):
      self._healthz()
    else:
      self._404()
    return

  def do_POST(self):
    self._check_user_access()

try:
  server = HTTPServer((LISTEN_ADDRESS, LISTEN_PORT), myHandler)
  print('Started httpserver on ' + LISTEN_ADDRESS + ':' + str(LISTEN_PORT) )

  server.serve_forever()

except KeyboardInterrupt:
  print('^C received, shutting down the web server')
  server.socket.close()
