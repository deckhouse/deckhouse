-- wrk script to benchmark GeoProxy/ext_proc with controllable X-Forwarded-For cardinality.
--
-- Usage:
--   # Pool of N IPs (repeats, simulates cache hit-rate)
--   wrk -t4 -c200 -d30s -s wrk-xff-pool.lua -- 10000
--
--   # Always the same IP (best-case cache)
--   wrk -t4 -c200 -d30s -s wrk-xff-pool.lua -- 1
--   wrk -t4 -c200 -d30s -s wrk-xff-pool.lua -- 1 fixed 188.40.167.82
--
--   # Random IP every request (worst-case cache)
--   wrk -t4 -c200 -d30s -s wrk-xff-pool.lua -- 0 random
--
-- Notes:
-- - Uses only X-Forwarded-For; Envoy usually derives x-envoy-external-address from it.
-- - IP generation is deterministic for the "pool" mode to avoid per-thread RNG skew.
--
local counter = 0
local poolSize = 10000
local mode = "pool" -- pool | random | fixed
local fixedIP = nil

function init(args)
  if args[1] ~= nil then
    poolSize = tonumber(args[1]) or poolSize
  end
  if args[2] ~= nil then
    mode = tostring(args[2])
  end
  if mode == "fixed" then
    fixedIP = args[3] or "188.40.167.82"
  end
  math.randomseed(os.time() + (poolSize or 0))
end

local function ipFromIndex(i)
  -- Simple LCG to spread indexes across IPv4 space.
  local x = (1103515245 * i + 12345) % 2147483648
  local a = (x % 223) + 1
  x = math.floor(x / 223)
  local b = x % 256
  x = math.floor(x / 256)
  local c = x % 256
  x = math.floor(x / 256)
  local d = (x % 254) + 1
  return string.format("%d.%d.%d.%d", a, b, c, d)
end

request = function()
  counter = counter + 1

  local ip
  if mode == "fixed" and fixedIP ~= nil then
    ip = fixedIP
  elseif mode == "random" or poolSize == 0 then
    ip = string.format(
      "%d.%d.%d.%d",
      math.random(1, 223),
      math.random(0, 255),
      math.random(0, 255),
      math.random(1, 254)
    )
  else
    local idx = (counter % poolSize) + 1
    ip = ipFromIndex(idx)
  end

  wrk.headers["X-Forwarded-For"] = ip
  return wrk.format("GET", "/")
end

