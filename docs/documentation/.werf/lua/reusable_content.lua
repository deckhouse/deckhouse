--[[
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
--]]


local cjson = require("cjson.safe")

local M = {}

local placeholder_pattern = '<script%s+type=(["\'])application/x%-module%-include%1%s*>(.-)</script>'
local static_attr_pattern_with_dot = '([%a]+)=([\"\'])%./static/([^\"\']+)%2'
local static_attr_pattern_plain = '([%a]+)=([\"\'])static/([^\"\']+)%2'
local markdown_link_pattern = "%[([^%]]+)%]%(([^)]+)%)"

local allowed_channels = {
  alpha = true,
  beta = true,
  ["early-access"] = true,
  stable = true,
  ["rock-solid"] = true,
  latest = true,
}

local function debug_enabled()
  return ngx.var.cookie_debug == "1"
end

local function debug_log(...)
  if debug_enabled() then
    ngx.log(ngx.NOTICE, "LUA reusable: ", ...)
  end
end

local function increment_counter(name, value)
  local dict = ngx.shared.reusable_content_metrics
  if not dict then
    return
  end

  local ok, err = dict:incr(name, value or 1, 0)
  if not ok and err then
    ngx.log(ngx.WARN, "failed to increment reusable content metric ", name, ": ", err)
  end
end

local function copy_headers(headers)
  for k, v in pairs(headers or {}) do
    local key = string.lower(k)
    if key ~= "content-length" and key ~= "transfer-encoding" and key ~= "connection" then
      ngx.header[k] = v
    end
  end
end

local function make_html_response(res, body)
  ngx.status = res.status
  copy_headers(res.header)
  ngx.header["Content-Length"] = nil
  ngx.print(body or res.body or "")
  return ngx.exit(res.status)
end

local function trim(value)
  if not value then
    return nil
  end

  return (value:gsub("^%s+", ""):gsub("%s+$", ""))
end

local function to_html_fallback(value)
  if not value or value == "" then
    return ""
  end

  local replaced = value:gsub(markdown_link_pattern, [[<a href="%2">%1</a>]])
  return "<div class=\"reusable-content-fallback\">" .. replaced .. "</div>"
end

local function normalize_prefix(prefix)
  local value = prefix or ""
  if value == "" then
    return ""
  end

  if not value:match("^/") then
    value = "/" .. value
  end

  return value:gsub("/+$", "")
end

local function build_include_url(cfg, module_prefix)
  local artifact = cfg.artifact
  if not artifact or not artifact:match("^[a-z0-9-][a-z0-9-./]*%.html$") then
    return nil, "invalid artifact"
  end

  local module_name = cfg.module
  if not module_name or not module_name:match("^[a-z0-9-]+$") then
    return nil, "invalid module"
  end

  local channel = cfg.channel or "stable"
  if not allowed_channels[channel] then
    return nil, "invalid channel"
  end

  local base = normalize_prefix(module_prefix)

  return string.format("%s/%s/%s/partials/%s", base, module_name, channel, artifact), nil
end

local function rewrite_static_links(body, cfg, public_prefix)
  local prefix = string.format(
    "%s/%s/%s/partials/static/",
    normalize_prefix(public_prefix),
    cfg.module,
    cfg.channel or "stable"
  )

  local function rewrite_attr(pattern, value)
    return (value:gsub(pattern, function(attr, quote, path)
      if attr ~= "src" and attr ~= "href" then
        return string.format('%s=%sstatic/%s%s', attr, quote, path, quote)
      end

      return string.format('%s=%s%s%s', attr, quote, prefix .. path, quote)
    end))
  end

  return rewrite_attr(static_attr_pattern_plain, rewrite_attr(static_attr_pattern_with_dot, body))
end

local function render_placeholder(script_body, fetch_prefix, public_prefix)
  increment_counter("reusable_content_placeholders_total", 1)
  debug_log("placeholder body=", tostring(script_body))

  local decoded, decode_err = cjson.decode(trim(script_body))
  if not decoded then
    increment_counter("reusable_content_render_errors_total", 1)
    ngx.log(ngx.WARN, "failed to decode module include placeholder: ", tostring(decode_err))
    debug_log("decode failed: ", tostring(decode_err))
    return ""
  end

  debug_log(
    "decoded placeholder module=", tostring(decoded.module),
    ", channel=", tostring(decoded.channel),
    ", artifact=", tostring(decoded.artifact),
    ", onError=", tostring(decoded.onError)
  )

  local include_url, url_err = build_include_url(decoded, fetch_prefix)
  if not include_url then
    increment_counter("reusable_content_render_errors_total", 1)
    ngx.log(ngx.WARN, "invalid module include placeholder: ", tostring(url_err))
    debug_log("invalid placeholder: ", tostring(url_err))
    if decoded.onError == "fallback" then
      increment_counter("reusable_content_fallbacks_total", 1)
      debug_log("using fallback for invalid placeholder")
      return to_html_fallback(decoded.fallback)
    end
    return ""
  end

  debug_log("include_url=", include_url)
  increment_counter("reusable_content_include_requests_total", 1)
  local res = ngx.location.capture(include_url)
  debug_log(
    "include response status=", tostring(res and res.status or "nil"),
    ", body_length=", tostring(res and res.body and #res.body or 0)
  )
  if not res or res.status >= 400 then
    increment_counter("reusable_content_render_errors_total", 1)
    ngx.log(ngx.WARN, "failed to load reusable content from ", include_url, ", status=", res and res.status or "nil")
    if decoded.onError == "fallback" then
      increment_counter("reusable_content_fallbacks_total", 1)
      debug_log("using fallback after include failure")
      return to_html_fallback(decoded.fallback)
    end
    return ""
  end

  debug_log("rewriting static links with public_prefix=", tostring(public_prefix))
  return rewrite_static_links(res.body or "", decoded, public_prefix)
end

local function render_placeholders(body, fetch_prefix, public_prefix)
  local parts = {}
  local last_index = 1

  while true do
    local start_pos, end_pos, _quote, script_body = body:find(placeholder_pattern, last_index)
    if not start_pos then
      parts[#parts + 1] = body:sub(last_index)
      break
    end

    parts[#parts + 1] = body:sub(last_index, start_pos - 1)
    parts[#parts + 1] = render_placeholder(script_body, fetch_prefix, public_prefix)
    last_index = end_pos + 1
  end

  return table.concat(parts)
end

function M.render(raw_location, fetch_prefix, public_prefix)
  debug_log(
    "render start raw_location=", tostring(raw_location),
    ", fetch_prefix=", tostring(fetch_prefix),
    ", public_prefix=", tostring(public_prefix)
  )
  local res = ngx.location.capture(raw_location)
  if not res then
    increment_counter("reusable_content_render_errors_total", 1)
    debug_log("raw location capture failed")
    return ngx.exit(ngx.HTTP_INTERNAL_SERVER_ERROR)
  end

  local content_type = (res.header and (res.header["Content-Type"] or res.header["content-type"])) or ""
  local body = res.body or ""
  debug_log(
    "raw response status=", tostring(res.status),
    ", content_type=", tostring(content_type),
    ", body_length=", tostring(#body)
  )
  if not content_type:find("text/html", 1, true) or not body:find("application/x-module-include", 1, true) then
    debug_log("response bypassed reusable content processing")
    return make_html_response(res, body)
  end

  increment_counter("reusable_content_pages_total", 1)
  local rendered = render_placeholders(body, fetch_prefix, public_prefix)
  debug_log("render completed")

  return make_html_response(res, rendered)
end

return M
