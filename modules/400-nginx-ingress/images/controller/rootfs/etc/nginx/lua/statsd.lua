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

  local var_server_name = ngx.var.server_name
  if var_server_name == "_" then
    _incr(buffer, "l#")
  else
    local content_kind
    local var_upstream_x_content_kind = ngx.var.upstream_x_content_kind
    local var_upstream_addr = ngx.var.upstream_addr
    local var_http_upgrade = ngx.var.http_upgrade
    local var_upstream_http_cache_control = ngx.var.upstream_http_cache_control
    local var_upstream_http_expires = ngx.var.upstream_http_expires
    if var_upstream_x_content_kind then
      content_kind = var_upstream_x_content_kind
    elseif not var_upstream_addr then
      content_kind = 'served-without-upstream'
    elseif var_http_upgrade then
      content_kind = string.lower(var_http_upgrade)
    elseif var_upstream_http_cache_control or var_upstream_http_expires then
      local cacheable = true
      if var_upstream_http_cache_control then
        if string.match(var_upstream_http_cache_control, "no-cache") or string.match(var_upstream_http_cache_control, "no-store") or string.match(var_upstream_http_cache_control, "private") then
          cacheable = false
        end
      end
      if var_upstream_http_expires then
        local var_upstream_http_expires_parsed = ngx.parse_http_time(var_upstream_http_expires)
        if not var_upstream_http_expires_parsed or var_upstream_http_expires_parsed <= ngx.time() then
          cacheable = false
        end
      end
      local var_upstream_http_vary = ngx.var.upstream_http_vary
      if var_upstream_http_vary and var_upstream_http_vary == "*" then
        cacheable = false
      end
      if ngx.var.upstream_http_set_cookie then
        cacheable = false
      end
  
      if cacheable then
        content_kind = 'cacheable'
      else
        content_kind = 'non-cacheable'
      end
    else
      content_kind = 'cache-headers-not-present'
    end
    ngx.var.content_kind = content_kind

    local var_namespace = ngx.var.namespace
    local overall_key = content_kind .. "#" .. var_namespace .. "#" .. var_server_name
    local detail_key = content_kind .. "#" .. var_namespace .. "#" .. ngx.var.ingress_name .. "#" .. ngx.var.service_name .. "#" .. ngx.var.service_port .. "#"  .. var_server_name .. "#" .. ngx.var.location_path
    local backend_key = var_namespace .. "#" .. ngx.var.ingress_name .. "#" .. ngx.var.service_name .. "#" .. ngx.var.service_port  .. "#" .. var_server_name .. "#" .. ngx.var.location_path

    -- requests
    local var_scheme = ngx.var.scheme
    local var_request_method = ngx.var.request_method
    _incr(buffer, "ao#" .. overall_key .. "#" .. var_scheme .. "#" .. var_request_method)
    _incr(buffer, "ad#" .. detail_key .. "#" .. var_scheme .. "#" .. var_request_method)

    -- responses
    local var_status = ngx.var.status
    _incr(buffer, "bo#" .. overall_key .. "#" .. var_status)
    _incr(buffer, "bd#" .. detail_key .. "#" .. var_status)

    -- request time
    local var_request_time = tonumber(ngx.var.request_time)
    _hist(buffer, "co#" .. overall_key, var_request_time)
    _hist(buffer, "cd#" .. detail_key, var_request_time)

    -- bytes sent
    local var_bytes_sent = ngx.var.bytes_sent
    _hist(buffer, "do#" .. overall_key, var_bytes_sent)
    _hist(buffer, "dd#" .. detail_key, var_bytes_sent)

    -- bytes received (according to https://serverfault.com/questions/346853/logging-request-response-size-in-access-log-of-nginx)
    local var_request_length = ngx.var.request_length
    _hist(buffer, "eo#" .. overall_key, var_request_length)
    _hist(buffer, "ed#" .. detail_key, var_request_length)

    -- upstreams
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
        _hist(buffer, "ka#" .. backend_key .. "#" .. backends[n], response_time)
      end
      ngx.var.total_upstream_response_time = upstream_response_time

      -- upstream response time
      _hist(buffer, "fo#" .. overall_key, upstream_response_time)
      _hist(buffer, "go#" .. overall_key, upstream_response_time)
      _hist(buffer, "fd#" .. detail_key, upstream_response_time)
      _hist(buffer, "gd#" .. detail_key, upstream_response_time)

      local upstream_redirects = 0
      for _ in string.gmatch(var_upstream_response_time, ":") do
        upstream_redirects = upstream_redirects + 1
      end

      local upstream_retries = upstream_requests - upstream_redirects - 1
      ngx.var.upstream_retries = upstream_retries
      if upstream_retries > 0 then
        -- upstream retries (count)
        _incr(buffer, "ho#" .. overall_key)
        _incr(buffer, "hd#" .. detail_key)

        -- upstream retries (sum)
        _count(buffer, "io#" .. overall_key, upstream_retries)
        _count(buffer, "id#" .. detail_key, upstream_retries)
      end

      for status in string.gmatch(ngx.var.upstream_status, "[%d]+") do
        -- responses (for each backend)
        _incr(buffer, "kb#" .. backend_key .. "#" .. backends[n] .. "#" .. string.sub(status, 1, 1))
      end

      for upstream_bytes_received in string.gmatch(ngx.var.upstream_bytes_received, "[%d]+") do
        -- upstream bytes received (for each backend)
        _count(buffer, "kc#" .. backend_key .. "#" .. backends[n], upstream_bytes_received)
      end
    end

    local geoip_latitude = tonumber(ngx.var.geoip_latitude)
    local geoip_longitude = tonumber(ngx.var.geoip_longitude)
    if geoip_latitude and geoip_longitude then
      GeoHash = require "geohash"
      GeoHash.precision(2)
      local geohash = GeoHash.encode(geoip_latitude, geoip_longitude)

      local place = "Unknown"
      local var_geoip_city = ngx.var.geoip_city
      local var_geoip_region_name = ngx.var.geoip_region_name
      local var_geoip_country_name = ngx.var.geoip_city_country_code
      if var_geoip_city then
        place = var_geoip_city
      elseif var_geoip_region_name then
        place = var_geoip_region_name
      elseif var_geoip_country_name then
        place = var_geoip_country_name
      end

      -- geohash
      _incr(buffer, "jo#" .. overall_key .. "#" .. geohash .. "#" .. place)
    end
  end

  buffer["len"] = nil
  ngx.timer.at(0, _send, buffer)
end

return _M
