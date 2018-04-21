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
  local var_status = ngx.var.status
  _incr(buffer, "response_status#" .. overall_key .. "#" .. var_status)
  _incr(buffer, "ns#response_status#" .. namespace_key .. "#" .. var_status)
  _incr(buffer, "ns_s#response_status#" .. namespace_service_key .. "#" .. var_status)
  _incr(buffer, "s#response_status#" .. server_key .. "#" .. var_status)

  -- request time
  local var_request_time = tonumber(ngx.var.request_time)
  _hist(buffer, "request_time#" .. overall_key, var_request_time)
  _hist(buffer, "ns#request_time#" .. namespace_key, var_request_time)
  _hist(buffer, "ns_s#request_time#" .. namespace_service_key, var_request_time)
  _hist(buffer, "s#request_time#" .. server_key, var_request_time)

  -- bytes sent
  local var_bytes_sent = ngx.var.bytes_sent
  _hist(buffer, "bytes_sent#" .. overall_key, var_bytes_sent)
  _hist(buffer, "ns#bytes_sent#" .. namespace_key, var_bytes_sent)
  _hist(buffer, "ns_s#bytes_sent#" .. namespace_service_key, var_bytes_sent)
  _hist(buffer, "s#bytes_sent#" .. server_key, var_bytes_sent)

  -- bytes received
  local var_bytes_received = ngx.var.bytes_received
  if var_bytes_received then
    _hist(buffer, "bytes_received#" .. overall_key, var_bytes_received)
    _hist(buffer, "ns#bytes_received#" .. namespace_key, var_bytes_received)
    _hist(buffer, "ns_s#bytes_received#" .. namespace_service_key, var_bytes_received)
    _hist(buffer, "s#bytes_received#" .. server_key, var_bytes_received)
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
      _hist(buffer, "ns_s_b#upstream_response_time#" .. namespace_service_key .. "#" .. backends[n], response_time)
      _hist(buffer, "s_b#upstream_response_time#" .. server_key .. "#" .. backends[n], response_time)
    end
    local upstream_redirects = 0
    for _ in string.gmatch(var_upstream_response_time, ":") do
      upstream_redirects = upstream_redirects + 1
    end

    for status in string.gmatch(ngx.var.upstream_status, "[%d]+") do
      -- response status (for each backend)
      _incr(buffer, "ns_s_b#response_status#" .. namespace_service_key .. "#" .. backends[n] .. "#" .. var_status)
      _incr(buffer, "s_b#response_status#" .. server_key .. "#" .. backends[n] .. "#" .. var_status)
    end

    -- upstream response time
    _hist(buffer, "upstream_response_time#" .. overall_key, upstream_response_time)
    _hist(buffer, "ns#upstream_response_time#" .. namespace_key, upstream_response_time)
    _hist(buffer, "ns_s#upstream_response_time#" .. namespace_service_key, upstream_response_time)
    _hist(buffer, "s#upstream_response_time#" .. server_key, upstream_response_time)

    -- upstream retries
    _count(buffer, "upstream_retries#" .. overall_key, upstream_requests - upstream_redirects)
    _count(buffer, "ns#upstream_retries#" .. namespace_key, upstream_requests - upstream_redirects)
    _count(buffer, "ns_s#upstream_retries#" .. namespace_service_key, upstream_requests - upstream_redirects)
    _count(buffer, "s#upstream_retries#" .. server_key, upstream_requests - upstream_redirects)

    -- upstream redirects
    _count(buffer, "upstream_redirects#" .. overall_key, upstream_redirects)
    _count(buffer, "ns#upstream_redirects#" .. namespace_key, upstream_redirects)
    _count(buffer, "ns_s#upstream_redirects#" .. namespace_service_key, upstream_redirects)
    _count(buffer, "s#upstream_redirects#" .. server_key, upstream_redirects)
  end
  
  buffer["len"] = nil
  ngx.timer.at(0, _send, buffer)
end

return _M
