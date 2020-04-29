local socket = ngx.socket.tcp
local parse_http_time = ngx.parse_http_time
local timer_at = ngx.timer.at
local timer_every = ngx.timer.every
local worker_id = ngx.worker.id
local running_timers = ngx.timer.running_count
local pending_timers = ngx.timer.pending_count

local log = ngx.log
local ERROR = ngx.ERR
local WARNING = ngx.WARN

local new_tab = require "table.new"
local clear_tab = require "table.clear"
local clone_tab = require "table.clone"
local nkeys = require "table.nkeys"
local insert_tab = table.insert
local concat_tab = table.concat

local match = string.match
local gmatch = string.gmatch
local sub = string.sub
local format = string.format

local GeoHash = require "geohash"
GeoHash.precision(2)
local geohash_encode = GeoHash.encode

local buffer = new_tab(200000, 0)
local buffer_size = 0

local max_buffer_size = 2012324 -- # ~2mb
local max_value_size = 4 -- # expected that single nginx worker can't receive more than 10k request/sec
local buffer_count_additional_chars = 4 -- 4 is ":" and "|c" and "\n" bytes count


-- _add() adds value to the metric
local function _add(metric, value)
  local data = buffer[metric]
  if data then
    buffer[metric] = data + value
    return
  end

  buffer[metric] = value
  buffer_size = buffer_size + metric:len() + max_value_size + buffer_count_additional_chars
end

-- _increment() adds one to the metric
local function _increment(metric)
  _add(metric, 1)
end

-- ### *_observe functions are generated for better performance ###

-- _time_observe() prepares histogram metrics for time buckets
local function _time_observe(metric, value)
  _add("s" .. metric, value)
  _increment("c" .. metric)

  if value <= 0.001 then _increment(metric .. "#0.001") return end
  if value <= 0.002 then _increment(metric .. "#0.002") return end
  if value <= 0.003 then _increment(metric .. "#0.003") return end
  if value <= 0.004 then _increment(metric .. "#0.004") return end
  if value <= 0.005 then _increment(metric .. "#0.005") return end
  if value <= 0.01 then _increment(metric .. "#0.01") return end
  if value <= 0.015 then _increment(metric .. "#0.015") return end
  if value <= 0.02 then _increment(metric .. "#0.02") return end
  if value <= 0.025 then _increment(metric .. "#0.025") return end
  if value <= 0.03 then _increment(metric .. "#0.03") return end
  if value <= 0.035 then _increment(metric .. "#0.035") return end
  if value <= 0.04 then _increment(metric .. "#0.04") return end
  if value <= 0.045 then _increment(metric .. "#0.045") return end
  if value <= 0.05 then _increment(metric .. "#0.05") return end
  if value <= 0.06 then _increment(metric .. "#0.06") return end
  if value <= 0.07 then _increment(metric .. "#0.07") return end
  if value <= 0.08 then _increment(metric .. "#0.08") return end
  if value <= 0.09 then _increment(metric .. "#0.09") return end
  if value <= 0.1 then _increment(metric .. "#0.1") return end
  if value <= 0.15 then _increment(metric .. "#0.15") return end
  if value <= 0.2 then _increment(metric .. "#0.2") return end
  if value <= 0.25 then _increment(metric .. "#0.25") return end
  if value <= 0.3 then _increment(metric .. "#0.3") return end
  if value <= 0.35 then _increment(metric .. "#0.35") return end
  if value <= 0.4 then _increment(metric .. "#0.4") return end
  if value <= 0.45 then _increment(metric .. "#0.45") return end
  if value <= 0.5 then _increment(metric .. "#0.5") return end
  if value <= 0.6 then _increment(metric .. "#0.6") return end
  if value <= 0.7 then _increment(metric .. "#0.7") return end
  if value <= 0.8 then _increment(metric .. "#0.8") return end
  if value <= 0.9 then _increment(metric .. "#0.9") return end
  if value <= 1 then _increment(metric .. "#1") return end
  if value <= 1.5 then _increment(metric .. "#1.5") return end
  if value <= 2 then _increment(metric .. "#2") return end
  if value <= 2.5 then _increment(metric .. "#2.5") return end
  if value <= 3 then _increment(metric .. "#3") return end
  if value <= 3.5 then _increment(metric .. "#3.5") return end
  if value <= 4 then _increment(metric .. "#4") return end
  if value <= 4.5 then _increment(metric .. "#4.5") return end
  if value <= 5 then _increment(metric .. "#5") return end
  if value <= 6 then _increment(metric .. "#6") return end
  if value <= 7 then _increment(metric .. "#7") return end
  if value <= 8 then _increment(metric .. "#8") return end
  if value <= 9 then _increment(metric .. "#9") return end
  if value <= 10 then _increment(metric .. "#10") return end
  if value <= 15 then _increment(metric .. "#15") return end
  if value <= 20 then _increment(metric .. "#20") return end
  if value <= 25 then _increment(metric .. "#25") return end
  if value <= 30 then _increment(metric .. "#30") return end
  if value <= 35 then _increment(metric .. "#35") return end
  if value <= 40 then _increment(metric .. "#40") return end
  if value <= 45 then _increment(metric .. "#45") return end
  if value <= 50 then _increment(metric .. "#50") return end
  if value <= 55 then _increment(metric .. "#55") return end
  if value <= 60 then _increment(metric .. "#60") return end
  if value <= 90 then _increment(metric .. "#90") return end
  if value <= 120 then _increment(metric .. "#120") return end
  if value <= 180 then _increment(metric .. "#180") return end
  if value <= 240 then _increment(metric .. "#240") return end
  if value <= 270 then _increment(metric .. "#270") return end
  if value <= 300 then _increment(metric .. "#300") return end
  if value <= 360 then _increment(metric .. "#360") return end
  if value <= 420 then _increment(metric .. "#420") return end
  if value <= 480 then _increment(metric .. "#480") return end
  if value <= 540 then _increment(metric .. "#540") return end
  if value <= 600 then _increment(metric .. "#600") return end
  if value <= 900 then _increment(metric .. "#900") return end
  if value <= 1200 then _increment(metric .. "#1200") return end
  if value <= 1500 then _increment(metric .. "#1500") return end
  if value <= 1800 then _increment(metric .. "#1800") return end
  if value <= 3600 then _increment(metric .. "#3600") return end
  _increment(metric .. "#" .. "+Inf")
end

-- _bytes_observe() prepares histogram metrics for bytes buckets
local function _bytes_observe(metric, value)
  value = tonumber(value)

  _add("s" .. metric, value)
  _increment("c" .. metric)

  if value <= 64 then _increment(metric .. "#64") return end
  if value <= 128 then _increment(metric .. "#128") return end
  if value <= 256 then _increment(metric .. "#256") return end
  if value <= 512 then _increment(metric .. "#512") return end
  if value <= 1024 then _increment(metric .. "#1024") return end
  if value <= 2048 then _increment(metric .. "#2048") return end
  if value <= 4096 then _increment(metric .. "#4096") return end
  if value <= 8192 then _increment(metric .. "#8192") return end
  if value <= 16384 then _increment(metric .. "#16384") return end
  if value <= 32768 then _increment(metric .. "#32768") return end
  if value <= 65536 then _increment(metric .. "#65536") return end
  if value <= 131072 then _increment(metric .. "#131072") return end
  if value <= 262144 then _increment(metric .. "#262144") return end
  if value <= 524288 then _increment(metric .. "#524288") return end
  if value <= 1048576 then _increment(metric .. "#1048576") return end
  if value <= 2097152 then _increment(metric .. "#2097152") return end
  if value <= 4194304 then _increment(metric .. "#4194304") return end
  if value <= 8388608 then _increment(metric .. "#8388608") return end
  if value <= 16777216 then _increment(metric .. "#16777216") return end
  if value <= 33554432 then _increment(metric .. "#33554432") return end
  if value <= 67108864 then _increment(metric .. "#67108864") return end
  if value <= 134217728 then _increment(metric .. "#134217728") return end
  if value <= 268435456 then _increment(metric .. "#268435456") return end
  if value <= 536870912 then _increment(metric .. "#536870912") return end
  if value <= 1073741824 then _increment(metric .. "#1073741824") return end
  if value <= 2147483648 then _increment(metric .. "#2147483648") return end
  if value <= 4294967296 then _increment(metric .. "#4294967296") return end
  _increment(metric .. "#" .. "+Inf")
end

-- _lowres_observe() prepares histogram metrics for lowres time buckets
local function _lowres_observe(metric, value)
  _add("s" .. metric, value)
  _increment("c" .. metric)

  if value <= 0.005 then _increment(metric .. "#0.005") return end
  if value <= 0.01 then _increment(metric .. "#0.01") return end
  if value <= 0.02 then _increment(metric .. "#0.02") return end
  if value <= 0.03 then _increment(metric .. "#0.03") return end
  if value <= 0.04 then _increment(metric .. "#0.04") return end
  if value <= 0.05 then _increment(metric .. "#0.05") return end
  if value <= 0.075 then _increment(metric .. "#0.075") return end
  if value <= 0.1 then _increment(metric .. "#0.1") return end
  if value <= 0.2 then _increment(metric .. "#0.2") return end
  if value <= 0.3 then _increment(metric .. "#0.3") return end
  if value <= 0.4 then _increment(metric .. "#0.4") return end
  if value <= 0.5 then _increment(metric .. "#0.5") return end
  if value <= 0.75 then _increment(metric .. "#0.75") return end
  if value <= 1 then _increment(metric .. "#1") return end
  if value <= 1.5 then _increment(metric .. "#1.5") return end
  if value <= 2 then _increment(metric .. "#2") return end
  if value <= 3 then _increment(metric .. "#3") return end
  if value <= 4 then _increment(metric .. "#4") return end
  if value <= 5 then _increment(metric .. "#5") return end
  if value <= 10 then _increment(metric .. "#10") return end
  _increment(metric .. "#" .. "+Inf")
end

-- fill_statsd_buffer() prepares statsd data metrics
local function fill_statsd_buffer()
  -- default values
  ngx.var.total_upstream_response_time = 0
  ngx.var.upstream_retries = 0

  local var_server_name = ngx.var.server_name:gsub("^*", "")

  if var_server_name == "_" then
    _increment("l#")
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

  local var_namespace = ngx.var.namespace
  local overall_key = content_kind .. "#" .. var_namespace .. "#" .. var_server_name
  local detail_key = content_kind .. "#" .. var_namespace .. "#" .. ngx.var.ingress_name .. "#" .. ngx.var.service_name .. "#" .. ngx.var.service_port .. "#"  .. var_server_name .. "#" .. ngx.var.location_path
  local backend_key = var_namespace .. "#" .. ngx.var.ingress_name .. "#" .. ngx.var.service_name .. "#" .. ngx.var.service_port  .. "#" .. var_server_name .. "#" .. ngx.var.location_path
  -- requests
  local var_scheme = ngx.var.scheme
  local var_request_method = ngx.var.request_method
  _increment("ao#" .. overall_key .. "#" .. var_scheme .. "#" .. var_request_method)
  _increment("ad#" .. detail_key .. "#" .. var_scheme .. "#" .. var_request_method)

  -- responses
  local var_status = ngx.var.status
  _increment("bo#" .. overall_key .. "#" .. var_status)
  _increment("bd#" .. detail_key .. "#" .. var_status)

  -- request time
  local var_request_time = tonumber(ngx.var.request_time)
  _time_observe("co#" .. overall_key, var_request_time)
  _time_observe("cd#" .. detail_key, var_request_time)

  -- bytes sent
  local var_bytes_sent = ngx.var.bytes_sent
  _bytes_observe("do#" .. overall_key, var_bytes_sent)
  _bytes_observe("dd#" .. detail_key, var_bytes_sent)

  -- bytes received (according to https://serverfault.com/questions/346853/logging-request-response-size-in-access-log-of-nginx)
  local var_request_length = ngx.var.request_length
  _bytes_observe("eo#" .. overall_key, var_request_length)
  _bytes_observe("ed#" .. detail_key, var_request_length)

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
      _lowres_observe("ka#" .. backend_key .. "#" .. backends[n], response_time)
    end
    ngx.var.total_upstream_response_time = upstream_response_time

    -- upstream response time
    _time_observe("fo#" .. overall_key, upstream_response_time)
    _time_observe("fd#" .. detail_key, upstream_response_time)
    _lowres_observe("go#" .. overall_key, upstream_response_time)
    _lowres_observe("gd#" .. detail_key, upstream_response_time)

    local upstream_redirects = 0
    for _ in gmatch(var_upstream_response_time, ":") do
      upstream_redirects = upstream_redirects + 1
    end

    local upstream_retries = upstream_requests - upstream_redirects - 1
    ngx.var.upstream_retries = upstream_retries
    if upstream_retries > 0 then
      -- upstream retries (count)
      _increment("ho#" .. overall_key)
      _increment("hd#" .. detail_key)

      -- upstream retries (sum)
      _add("io#" .. overall_key, upstream_retries)
      _add("id#" .. detail_key, upstream_retries)
    end

    n = 0
    for status in gmatch(ngx.var.upstream_status, "[%d]+") do
      -- responses (for each backend)
      n = n + 1
      _increment("kb#" .. backend_key .. "#" .. backends[n] .. "#" .. sub(status, 1, 1))
    end

    n = 0
    for upstream_bytes_received in gmatch(ngx.var.upstream_bytes_received, "[%d]+") do
      -- upstream bytes received (for each backend)
      n = n + 1
      _add("kc#" .. backend_key .. "#" .. backends[n], upstream_bytes_received)
    end
  end

  local geoip_latitude = tonumber(ngx.var.geoip_latitude)
  local geoip_longitude = tonumber(ngx.var.geoip_longitude)
  if geoip_latitude and geoip_longitude then
    local geohash = geohash_encode(geoip_latitude, geoip_longitude)

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
    _increment("jo#" .. overall_key .. "#" .. geohash .. "#" .. place)
  end
end

-- send() sends buffer data to statsd exporter via unixgram socket
local function send(premature)
  if premature then
    return
  end

  if buffer_size == 0 then
    return
  end

  -- DEBUG LOG
  -- log(WARNING, format("--------- Buffer size: %s bytes ---------\n", tostring(buffer_size)))

  local current_buffer = clone_tab(buffer)
  clear_tab(buffer)
  buffer_size = 0

  local lines_to_send = new_tab(nkeys(current_buffer) + 2, 0)
  for k, v in pairs(current_buffer) do
    insert_tab(lines_to_send, k .. ":" .. v .. "|c\n")
  end

  local id = tostring(worker_id())
  insert_tab(lines_to_send, "nr#" .. id .. ":" .. tostring(running_timers()) .. "|g\n")
  insert_tab(lines_to_send, "np#" .. id .. ":" .. tostring(pending_timers()) .. "|g\n")

  -- DEBUG LOG
  -- log(WARNING, format("Data: \n%s\n", concat_tab(lines_to_send)))

  local sock = socket()
  local ok, err = sock:connect("127.0.0.1", "4333")
  if not ok then
    log(ERROR, format("failed to connect to the tcp socket: %s", tostring(err)))
    return
  end
  sock:settimeout(60000) -- 1 min timeout

  ok, err = sock:send(lines_to_send)
  if not ok then
    log(ERROR, format("error while sending statsd data via tcp socket: %s", tostring(err)))
  end
  sock:close()
end

local _M = {}

-- init_worker() used at init_worker_by_lua_block stage to send buffer data to statsd-exporter
function _M.init_worker()
  local _, err = timer_every(2, send)
  if err then
    log(ERROR, format("error while sending statsd data: %s", tostring(err)))
  end
end

-- call() used at log_by_lua stage to save request data to the buffer
function _M.call()
  if buffer_size >= max_buffer_size then
    log(WARNING, "statsd buffer is full!")
    _increment("m#")

    -- clear the buffer and send data asynchronous
    timer_at(0, send)
  end

  fill_statsd_buffer()
end

return _M
