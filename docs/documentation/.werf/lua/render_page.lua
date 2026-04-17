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

local reusable_content = require("reusable_content")

local raw_uri = ngx.var.reusable_content_raw_uri
if not raw_uri or raw_uri == "" then
  ngx.log(ngx.ERR, "reusable content raw uri is not configured")
  return ngx.exit(ngx.HTTP_INTERNAL_SERVER_ERROR)
end

return reusable_content.render(
  raw_uri,
  ngx.var.reusable_content_fetch_prefix or "",
  ngx.var.reusable_content_public_prefix or ""
)
