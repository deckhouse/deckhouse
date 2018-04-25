local _M = {}

local function _send(premature, buffer)
  if premature then
    return
  end

  local udp = ngx.socket.udp()
  udp:setpeername("127.0.0.1", 9125)
  udp:send(buffer)
  udp:close()
end

local function _write(buffer, data)
  if buffer["len"] + #data > 1472 then
    ngx.timer.at(0, _send, table.concat(buffer))

    buffer["len"] = 0
    local n = #buffer
    while n >= 1 do
      buffer[n] = nil
      n = n - 1
    end
  end

  table.insert(buffer, data)
  buffer["len"] = buffer["len"] + #data
end

local function _hist(buffer, metric, value)
  -- prometheus statsd exporter divides all values by 1000
  _write(buffer, metric .. ":" .. value * 1000 .. "|h\n")
end

local function _count(buffer, metric, value)
  _write(buffer, metric .. ":" .. value .. "|c\n")
end

local function _incr(buffer, metric)
  _count(buffer, metric, 1)
end

function _M.call()
  local buffer = {}
  buffer["len"] = 0

  local var_namespace = ngx.var.namespace
  local var_server_name = ngx.var.server_name

  local overall_key = ngx.var.scheme .. "#" .. var_namespace .. "#" .. var_server_name
  local namespace_key = var_namespace .. "#" .. ngx.var.ingress_name
  local namespace_service_key = namespace_key .. "#" .. ngx.var.service_name .. "#" .. ngx.var.service_port
  local server_key = var_server_name .. "#" .. ngx.var.location_path

  -- response status
  local var_status = string.sub(ngx.var.status,1, 1) .. "xx"
  _incr(buffer, "a#" .. overall_key .. "#" .. var_status)
  _incr(buffer, "b#" .. namespace_key .. "#" .. var_status)
  _incr(buffer, "c#" .. namespace_service_key .. "#" .. var_status)
  _incr(buffer, "d#" .. server_key .. "#" .. var_status)

  -- request time
  local var_request_time = tonumber(ngx.var.request_time)
  _hist(buffer, "e#" .. overall_key, var_request_time)
  _hist(buffer, "f#" .. namespace_key, var_request_time)
  _hist(buffer, "g#" .. namespace_service_key, var_request_time)
  _hist(buffer, "h#" .. server_key, var_request_time)

  -- bytes sent
  local var_bytes_sent = ngx.var.bytes_sent
  _hist(buffer, "i#" .. overall_key, var_bytes_sent)
  _hist(buffer, "j#" .. namespace_key, var_bytes_sent)
  _hist(buffer, "k#" .. namespace_service_key, var_bytes_sent)
  _hist(buffer, "l#" .. server_key, var_bytes_sent)

  -- bytes received (according to https://serverfault.com/questions/346853/logging-request-response-size-in-access-log-of-nginx)
  local var_bytes_received = ngx.var.request_length
  if var_bytes_received then
    _hist(buffer, "m#" .. overall_key, var_bytes_received)
    _hist(buffer, "n#" .. namespace_key, var_bytes_received)
    _hist(buffer, "o#" .. namespace_service_key, var_bytes_received)
    _hist(buffer, "p#" .. server_key, var_bytes_received)
  end

  -- upstreams
  local var_upstream_addr = ngx.var.upstream_addr
  if var_upstream_addr then
    local backends = {}
    for backend in string.gmatch(var_upstream_addr, "([%d.]+):") do
      table.insert(backends, backend)
    end

    local n = 0
    local var_upstream_response_time = ngx.var.upstream_response_time
    local upstream_response_time = 0.0
    local upstream_requests = 0
    for t in string.gmatch(var_upstream_response_time, "[%d.]+") do
      local response_time = tonumber(t)
      n = n + 1

      upstream_response_time = upstream_response_time + response_time
      upstream_requests = upstream_requests + 1

      -- upstream response time (for each backend)
      _hist(buffer, "q#" .. namespace_service_key .. "#" .. backends[n], response_time)
      _hist(buffer, "r#" .. server_key .. "#" .. backends[n], response_time)
    end
    local upstream_redirects = 0
    for _ in string.gmatch(var_upstream_response_time, ":") do
      upstream_redirects = upstream_redirects + 1
    end

    for status in string.gmatch(ngx.var.upstream_status, "[%d]+") do
      -- response status (for each backend)
      if string.len(status)==3 then
        status = (string.sub(status,1, 1) .. "xx")
      end
      _incr(buffer, "s#" .. namespace_service_key .. "#" .. backends[n] .. "#" .. status)
      _incr(buffer, "t#" .. server_key .. "#" .. backends[n] .. "#" .. status)
    end

    -- upstream response time
    _hist(buffer, "u#" .. overall_key, upstream_response_time)
    _hist(buffer, "v#" .. namespace_key, upstream_response_time)
    _hist(buffer, "w#" .. namespace_service_key, upstream_response_time)
    _hist(buffer, "x#" .. server_key, upstream_response_time)

    -- upstream retries
    _count(buffer, "y#" .. overall_key, upstream_requests - upstream_redirects)
    _count(buffer, "z#" .. namespace_key, upstream_requests - upstream_redirects)
    _count(buffer, "0#" .. namespace_service_key, upstream_requests - upstream_redirects)
    _count(buffer, "1#" .. server_key, upstream_requests - upstream_redirects)

    -- upstream redirects
    _count(buffer, "2#" .. overall_key, upstream_redirects)
    _count(buffer, "3#" .. namespace_key, upstream_redirects)
    _count(buffer, "4#" .. namespace_service_key, upstream_redirects)
    _count(buffer, "5#" .. server_key, upstream_redirects)
  end

  buffer["len"] = nil
  ngx.timer.at(0, _send, buffer)
end

return _M
