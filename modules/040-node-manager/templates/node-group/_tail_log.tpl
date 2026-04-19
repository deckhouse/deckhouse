{{- define "node_group_tail_log_py" -}}
import os
import sys
import time

try:
    from http.server import BaseHTTPRequestHandler, HTTPServer
    from socketserver import ThreadingMixIn
except ImportError:
    from BaseHTTPServer import BaseHTTPRequestHandler, HTTPServer
    from SocketServer import ThreadingMixIn

LOG_FILE = sys.argv[1]
HOST = "0.0.0.0"
PORT = 8000

class ThreadedHTTPServer(ThreadingMixIn, HTTPServer): pass

class LogStreamHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-Type", "text/plain; charset=utf-8")
        self.end_headers()
        try:
            with open(LOG_FILE, "rb") as f:
                self.wfile.write(b"".join(f.read().splitlines(True)[-100:]))
                self.wfile.flush()
                f.seek(0, 2)

                while True:
                    curr_pos = f.tell()
                    line = f.readline()
                    if line:
                        self.wfile.write(line)
                        self.wfile.flush()
                    else:
                        try:
                            if os.stat(LOG_FILE).st_size < curr_pos:
                                f.seek(0)
                        except OSError:
                            pass
                        time.sleep(0.1)
        except Exception:
            pass

if __name__ == "__main__":
    print("Streaming {} on 0.0.0.0:{}".format(LOG_FILE, PORT))
    server = ThreadedHTTPServer(("0.0.0.0", PORT), LogStreamHandler)
    server.serve_forever()
{{ end }}
