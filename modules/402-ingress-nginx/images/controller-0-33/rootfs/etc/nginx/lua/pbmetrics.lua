local socket = ngx.socket.tcp
local parse_http_time = ngx.parse_http_time
-- local timer_at = ngx.timer.at
local timer_every = ngx.timer.every
-- local worker_id = ngx.worker.id
-- local running_timers = ngx.timer.running_count
-- local pending_timers = ngx.timer.pending_count
local now = ngx.now
local update_time = ngx.update_time

local log = ngx.log
local ERROR = ngx.ERR
local WARNING = ngx.WARN

local get_env = os.getenv

local new_tab = require "table.new"
local clear_tab = require "table.clear"
local clone_tab = require "table.clone"
local nkeys = require "table.nkeys"
local remove = table.remove
local insert_tab = table.insert

local match = string.match
local gmatch = string.gmatch
local sub = string.sub
local format = string.format

local iconv = require "iconv"
local utf8enc = iconv.new("utf-8", "latin1")

local GeoHash = require "geohash"
GeoHash.precision(2)
local geohash_encode = GeoHash.encode

local buffer = new_tab(200000, 0)
local debug_enabled = get_env("LUA_DEBUG")
local use_geoip2 = get_env("LUA_USE_GEOIP2")

-- setup protobuf
local pb = require "pb"
local protoc = require "protoc"
local pbuff = require "pb.buffer"

local p = protoc.new()

p:load([[
syntax="proto3";

package proto;

message HistogramMessage {
    int32 MappingIndex = 1;
    repeated string Labels = 2;
    map<string, uint64> Buckets = 3;
    double Sum = 4;
    uint64 Count = 5;
}

message CounterMessage {
    int32 MappingIndex = 1;
    repeated string Labels = 2;
    uint64 Value = 3;
}

message GaugeMessage {
    int32 MappingIndex = 1;
    repeated string Labels = 2;
    double Value = 3;
}
]])

local _HISTOGRAM_TYPE = 1
local _GAUGE_TYPE = 2
local _COUNTER_TYPE = 3

local function encode_buffer(buf, type, bytes)
  buf:pack("u", type)
  buf:pack("s", bytes)
end

local function protohist(buf, value)
  local bytes = pb.encode("proto.HistogramMessage", value)
  encode_buffer(buf, _HISTOGRAM_TYPE, bytes)
end

local function protogauge(buf, value)
  local bytes = pb.encode("proto.GaugeMessage", value)
  encode_buffer(buf, _GAUGE_TYPE, bytes)
end

local function protocounter(buf, value)
  local bytes = pb.encode("proto.CounterMessage", value)
  encode_buffer(buf, _COUNTER_TYPE, bytes)
end

local function _extract_labels(line)
  local t = {}
  for token in gmatch(line, "[^#]+") do
    t[#t + 1] = token
  end
  remove(t, 1)
  return t
end

-- _add() adds value to the metric
local function _add(metrichash, mapping, value)
  local metric_data = buffer[metrichash] or {MappingIndex = mapping, Value = 0, Labels = _extract_labels(metrichash)}
  metric_data["Value"] = metric_data["Value"] + value
  buffer[metrichash] = metric_data
end

-- _increment() adds one to the metric
local function _increment(metrichash, mapping)
  _add(metrichash, mapping, 1)
end

local function _observe(buckets, metrichash, mapping, value)
  local metric_data = buffer[metrichash] or {MappingIndex = mapping, Sum = 0, Count = 0, Labels = _extract_labels(metrichash)}
  metric_data["Sum"] = metric_data["Sum"] + value
  metric_data["Count"] = metric_data["Count"] + 1

  if not metric_data["Buckets"] then
    metric_data["Buckets"] = {}
  end

  for _, bucket in pairs(buckets) do
    if value <= bucket then
      local bucket_name = tostring(bucket)
      metric_data["Buckets"][bucket_name] = (metric_data["Buckets"][bucket_name] or 0) + 1
      break
    end
  end
  buffer[metrichash] = metric_data
end

local _TIME_BUCKETS = {0.001, 0.002, 0.003, 0.004, 0.005, 0.01, 0.015, 0.02, 0.025, 0.03, 0.035, 0.04, 0.045, 0.05, 0.06, 0.07, 0.08, 0.09, 0.1, 0.15, 0.2, 0.25, 0.3, 0.35, 0.4, 0.45, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 1.5, 2, 2.5, 3, 3.5, 4, 4.5, 5, 6, 7, 8, 9, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55, 60, 90, 120, 180, 240, 270, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600}

-- _time_observe() prepares histogram metrics for time buckets
local function _time_observe(metrichash, mapping, value)
  _observe(_TIME_BUCKETS, metrichash, mapping, value)
end


local _BYTES_BUCKETS = {64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536, 131072, 262144, 524288, 1048576, 2097152, 4194304, 8388608, 16777216, 33554432, 67108864, 134217728, 268435456, 536870912, 1073741824, 2147483648, 4294967296}

-- _bytes_observe() prepares histogram metrics for bytes buckets
local function _bytes_observe(metrichash, mapping, value)
  _observe(_BYTES_BUCKETS, metrichash, mapping, tonumber(value))
end

local _LOWRES_BUCKETS = {0.005, 0.01, 0.02, 0.03, 0.04, 0.05, 0.075, 0.1, 0.2, 0.3, 0.4, 0.5, 0.75, 1.0, 1.5, 2, 3, 4, 5, 10}

-- _lowres_observe() prepares histogram metrics for lowres time buckets
local function _lowres_observe(metrichash, mapping, value)
  _observe(_LOWRES_BUCKETS, metrichash, mapping, value)
end


local function _increment_geohash(overall_key, geoip_latitude, geoip_longitude, var_geoip_city, var_geoip_region_name, var_geoip_country_name)
  local geoip_latitude = tonumber(geoip_latitude)
  local geoip_longitude = tonumber(geoip_longitude)

  if geoip_latitude and geoip_longitude then
    local geohash = geohash_encode(geoip_latitude, geoip_longitude)

    local place = "Unknown"
    if var_geoip_city then
      place = var_geoip_city
    elseif var_geoip_region_name then
      place = var_geoip_region_name
    elseif var_geoip_country_name then
      place = var_geoip_country_name
    end

    local coded_place, err = utf8enc:iconv(place)
    if err then
      coded_place = place -- Try to send place as it is, exporter prepare propper metric for invalid place name
    end

    -- geohash
    _increment("c21#" .. overall_key .. "#" .. geohash .. "#" .. coded_place, 21)
  end
end

-- fill_buffer() prepares statsd data metrics
local function fill_buffer()
  local start_time = now()

  -- default values
  ngx.var.total_upstream_response_time = 0
  ngx.var.upstream_retries = 0

  local var_server_name = ngx.var.server_name:gsub("^*", "")

  if var_server_name == "_" then
    _increment("c22", 22)
    return
  end

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
      if match(var_upstream_http_cache_control, "no-cache") or match(var_upstream_http_cache_control, "no-store") or match(var_upstream_http_cache_control, "private") then
        cacheable = false
      end
    end

    if var_upstream_http_expires then
      local var_upstream_http_expires_parsed = parse_http_time(var_upstream_http_expires)
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

  local var_namespace = ngx.var.namespace == "" and "-" or ngx.var.namespace
  local var_ingress_name = ngx.var.ingress_name == "" and "-" or ngx.var.ingress_name
  local var_service_name = ngx.var.service_name == "" and "-" or ngx.var.service_name
  local var_service_port = ngx.var.service_port == "" and "-" or ngx.var.service_port

  local overall_key = content_kind .. "#" .. var_namespace .. "#" .. var_server_name
  local detail_key = content_kind .. "#" .. var_namespace .. "#" .. var_ingress_name .. "#" .. var_service_name .. "#" .. var_service_port .. "#"  .. var_server_name .. "#" .. ngx.var.location_path
  local backend_key = var_namespace .. "#" .. var_ingress_name .. "#" .. var_service_name .. "#" .. var_service_port  .. "#" .. var_server_name .. "#" .. ngx.var.location_path
  -- requests
  local var_scheme = ngx.var.scheme
  local var_request_method = ngx.var.request_method
  _increment("c00#" .. overall_key .. "#" .. var_scheme .. "#" .. var_request_method, 0)
  _increment("c01#" .. detail_key .. "#" .. var_scheme .. "#" .. var_request_method, 1)

  -- responses
  local var_status = ngx.var.status
  _increment("c02#" .. overall_key .. "#" .. var_status, 2)
  _increment("c03#" .. detail_key .. "#" .. var_status, 3)

  -- request time
  local var_request_time = tonumber(ngx.var.request_time)
  _time_observe("h04#" .. overall_key, 4, var_request_time)
  _time_observe("h05#" .. detail_key, 5, var_request_time)

  -- bytes sent
  local var_bytes_sent = ngx.var.bytes_sent
  _bytes_observe("h06#" .. overall_key, 6, var_bytes_sent)
  _bytes_observe("h07#" .. detail_key, 7, var_bytes_sent)

  -- bytes received (according to https://serverfault.com/questions/346853/logging-request-response-size-in-access-log-of-nginx)
  local var_request_length = ngx.var.request_length
  _bytes_observe("h08#" .. overall_key, 8, var_request_length)
  _bytes_observe("h09#" .. detail_key, 9, var_request_length)

  -- upstreams
  if var_upstream_addr then
    local backends = {}
    for backend in gmatch(var_upstream_addr, "([%d.]+):") do
      insert_tab(backends, backend)
    end

    local n = 0
    local var_upstream_response_time = ngx.var.upstream_response_time
    local upstream_response_time = 0.0
    local upstream_requests = 0
    for t in gmatch(var_upstream_response_time, "[%d.]+") do
      local response_time = tonumber(t)
      n = n + 1

      upstream_response_time = upstream_response_time + response_time
      upstream_requests = upstream_requests + 1

      -- upstream response time (for each backend)
      _lowres_observe("h18#" .. backend_key .. "#" .. backends[n], 18, response_time)
    end
    ngx.var.total_upstream_response_time = upstream_response_time

    -- upstream response time
    _time_observe("h10#" .. overall_key, 10, upstream_response_time)
    _time_observe("h11#" .. detail_key, 11, upstream_response_time)
    _lowres_observe("h12#" .. overall_key, 12, upstream_response_time)
    _lowres_observe("h13#" .. detail_key, 13, upstream_response_time)

    local upstream_redirects = 0
    for _ in gmatch(var_upstream_response_time, ":") do
      upstream_redirects = upstream_redirects + 1
    end

    local upstream_retries = upstream_requests - upstream_redirects - 1
    ngx.var.upstream_retries = upstream_retries
    if upstream_retries > 0 then
      -- upstream retries (count)
      _increment("c14#" .. overall_key, 14)
      _increment("c15#" .. detail_key, 15)

      -- upstream retries (sum)
      _add("g16#" .. overall_key, 16, upstream_retries)
      _add("g17#" .. detail_key, 17, upstream_retries)
    end

    n = 0
    for status in gmatch(ngx.var.upstream_status, "[%d]+") do
      -- responses (for each backend)
      n = n + 1
      _increment("c19#" .. backend_key .. "#" .. backends[n] .. "#" .. sub(status, 1, 1), 19)
    end

    n = 0
    for upstream_bytes_received in gmatch(ngx.var.upstream_bytes_received, "[%d]+") do
      -- upstream bytes received (for each backend)
      n = n + 1
      _add("g20#" .. backend_key .. "#" .. backends[n], 20, upstream_bytes_received)
    end
  end

  if use_geoip2 then
    _increment_geohash(overall_key, ngx.var.geoip2_latitude, ngx.var.geoip2_longitude, ngx.var.geoip2_city, ngx.var.geoip2_region_name, ngx.var.geoip2_city_country_code)
  else
    _increment_geohash(overall_key, ngx.var.geoip_latitude, ngx.var.geoip_longitude, ngx.var.geoip_city, ngx.var.geoip_region_name, ngx.var.geoip_city_country_code)
  end

  if debug_enabled then
      update_time()
      log(WARNING, format("lua parse seconds: %s", tostring(now() - start_time)))
  end
end

-- send() sends buffer data to statsd exporter via unixgram socket
local function send(premature)
  if premature then
    return
  end

  if nkeys(buffer) == 0 then
    return
  end

  local start_time = now()

  local current_buffer = clone_tab(buffer)
  clear_tab(buffer)

  local pbbuff = pbuff.new()
  for k, v in pairs(current_buffer) do
    local metric_type = k:sub(1, 1)
    if metric_type == "g" then
      protogauge(pbbuff, v)
    elseif metric_type == "c" then
      protocounter(pbbuff, v)
    elseif metric_type == "h" then
      protohist(pbbuff, v)
    end
  end

  local sock = socket()
  local ok, err = sock:connect("127.0.0.1", "9090")
  if not ok then
    log(ERROR, format("failed to connect to the tcp socket, metrcis buffer will be lost: %s", tostring(err)))
    return
  end
  sock:settimeout(60000) -- 1 min timeout

  ok, err = sock:send(pbbuff:result())
  if not ok then
    log(ERROR, format("error while sending statsd data via tcp socket: %s", tostring(err)))
  end
  sock:close()

  if debug_enabled then
      update_time()
      log(WARNING, format("lua send seconds: %s", tostring(now() - start_time)))
  end
end

local _M = {}

-- init_worker() used at init_worker_by_lua_block stage to send buffer data to statsd-exporter
function _M.init_worker()
  local _, err = timer_every(1, send)
  if err then
    log(ERROR, format("error while sending statsd data: %s", tostring(err)))
  end
end

-- call() used at log_by_lua stage to save request data to the buffer
function _M.call()
  fill_buffer()
end

return _M
