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

local dict = ngx.shared.reusable_content_metrics

ngx.header["Content-Type"] = "text/plain; charset=utf-8"

if not dict then
  ngx.say("# reusable content metrics are disabled")
  return
end

local metrics = {
  "reusable_content_pages_total",
  "reusable_content_placeholders_total",
  "reusable_content_include_requests_total",
  "reusable_content_render_errors_total",
  "reusable_content_fallbacks_total",
}

for _, name in ipairs(metrics) do
  ngx.say(name, " ", dict:get(name) or 0)
end
