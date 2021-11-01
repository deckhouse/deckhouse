local GeoHash = {}

--[[
-- Private Attributes
]]--

local _map = {}
      _map['0'] = '00000'
      _map['1'] = '00001'
      _map['2'] = '00010'
      _map['3'] = '00011'
      _map['4'] = '00100'
      _map['5'] = '00101'
      _map['6'] = '00110'
      _map['7'] = '00111'
      _map['8'] = '01000'
      _map['9'] = '01001'
      _map['b'] = '01010'
      _map['c'] = '01011'
      _map['d'] = '01100'
      _map['e'] = '01101'
      _map['f'] = '01110'
      _map['g'] = '01111'
      _map['h'] = '10000'
      _map['j'] = '10001'
      _map['k'] = '10010'
      _map['m'] = '10011'
      _map['n'] = '10100'
      _map['p'] = '10101'
      _map['q'] = '10110'
      _map['r'] = '10111'
      _map['s'] = '11000'
      _map['t'] = '11001'
      _map['u'] = '11010'
      _map['v'] = '11011'
      _map['w'] = '11100'
      _map['x'] = '11101'
      _map['y'] = '11110'
      _map['z'] = '11111'

local _precision = nil
local _digits    = 0

--[[
-- Private Methods
]]--

local function _decode(coord, min, max)
    local mid = 0.0
    local val = 0.0
    for i = 1, #coord do
        if coord:sub(i, i) == '1' then
            min = mid
            val = (mid + max) / 2
            mid = val
        else
            max = mid
            val = (mid + min) / 2
            mid = val
        end
    end
    -- We want number of decimals according to hash length
    val = tonumber(string.format("%.".. (#coord / 5) .. "f", val))
    return val
end

local function _encode(coord, min, max)
    local mid =   0.0
    local x   =   0.0
    local y   =   0.0
    local p   = ((_precision or _digits) * 5)
    local result = ''
    for i = 1, p do
        if coord <= max and coord >= mid then
            result = result .. '1'
            x = mid
            y = max
        else
            result = result .. '0'
            x = min
            y = mid
        end
        min = x
        mid = x + ((y - x) / 2)
        max = y
    end
    return result
end

local function _merge(latbin, longbin)
    local res = ''
    for i = 1, #latbin do
        res = res .. longbin:sub(i, i)  .. latbin:sub(i, i)
    end
    return res
end

local function _swap(tbl)
    local table = {}
    for key, val in pairs(tbl) do
        table[val] = key
    end
    return table
end

local function _translate(bstr)
    local hash = ''
    local t = _swap(_map)
    for i = 1, #bstr, 5 do
        hash = hash .. t[bstr:sub(i, i + 4)]
    end
    return hash
end

local function _decimals(lat, long)
     local d1 = tostring(string.match(tostring(lat), "%d+.(%d+)") or '')
     local d2 = tostring(string.match(tostring(long), "%d+.(%d+)") or '')
     local ret = #d2
     if #d1 > #d2 then
         ret = #d1
     elseif #d1 == 0 and #d2 == 0 then
         -- if no digits default to 2
         ret = 2
     end
     return ret
end

--[[
-- Public Methods
]]--

function GeoHash.decode(hash)
    local bin  = ''
    local long = ''
    local lat  = ''
    local c    = ''
    -- Convert hash to binary string
    for i = 1, #hash do
        c = hash:sub(i, i)
        if _map[c] ~= nil then
            bin = bin .. _map[c]
        else
            -- Invalid hash character, return nil
            return nil, nil
        end
    end
    -- Split binary string into latitude and longitude parts
    for i = 1, #bin do
        c = bin:sub(i, i)
        if i % 2 == 0 then
            lat = lat .. c
        else
            long = long .. c
        end
    end
    return _decode(lat, -90.0, 90.0), _decode(long, -180.0, 180.0)
end

function GeoHash.encode(lat, long)
    -- Find precision
    _digits = _decimals(lat, long)
    -- Translate coordinates to binary string format
    local a = _encode(lat, -90.0, 90.0)
    local b = _encode(long, -180.0, 180.0)
    -- Merge the two binary string
    local binstr = _merge(a, b)
    -- Calculate GeoHash for binary string
    return _translate(binstr)
end

function GeoHash.precision(p)
    _precision = p
end

return GeoHash
