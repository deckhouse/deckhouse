worker_processes auto;
error_log /dev/stderr warn;
pid /opt/nginx-static/logs/nginx.pid;

timer_resolution 100ms;
worker_cpu_affinity auto;
worker_rlimit_nofile 101000;

worker_shutdown_timeout 300s;

events {
  worker_connections 100000;
  multi_accept on;
}

http {
  access_log off;

  server {
    server_name _;
    listen 127.0.0.1:10253;

    location /healthz {
      return 200;
    }

    location /nginx_status {
      stub_status on;
    }
  }
}

stream {
  proxy_next_upstream_tries 10;
  proxy_connect_timeout 2s;
  proxy_timeout 12h;
  proxy_protocol on;
  log_format proxy '$remote_addr [$time_local] '
                 '$protocol $status $bytes_sent $bytes_received '
                 '$session_time "$upstream_addr" '
                 '"$upstream_bytes_sent" "$upstream_bytes_received" "$upstream_connect_time"';
  access_log /dev/stdout proxy;

  upstream http {
    server controller-${CONTROLLER_NAME}-failover:80 max_fails=0;
  }

  upstream https {
    server controller-${CONTROLLER_NAME}-failover:443 max_fails=0;
  }

  server {
    listen 169.254.20.11:1081 so_keepalive=off reuseport;
    proxy_pass http;
  }

  server {
    listen 169.254.20.11:1444 so_keepalive=off reuseport;
    proxy_pass https;
  }
}
