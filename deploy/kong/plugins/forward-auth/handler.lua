local http = require "resty.http"

local ForwardAuthHandler = {}

ForwardAuthHandler.PRIORITY = 1000
ForwardAuthHandler.VERSION = "1.0.0"

function ForwardAuthHandler:access(conf)
  local client = http.new()
  client:set_timeouts(10000, 10000, 10000) -- 10s timeouts

  local request_headers = kong.request.get_headers()
  
  -- Pass along authentication-related headers
  local auth_headers = {}
  for k, v in pairs(request_headers) do
    local lower_k = string.lower(k)
    if lower_k == "cookie" or lower_k == "authorization" or lower_k == "x-forwarded-proto" or lower_k == "x-forwarded-host" or lower_k == "x-forwarded-for" then
      auth_headers[k] = v
    end
  end

  -- Provide original request context headers to the auth outpost
  auth_headers["x-original-uri"] = kong.request.get_path_with_query()
  auth_headers["x-original-method"] = kong.request.get_method()

  local res, err = client:request_uri(conf.address, {
    method = conf.http_method or "GET",
    headers = auth_headers,
    ssl_verify = false,
  })

  if not res then
    kong.log.err("forward-auth subrequest failed: ", err)
    return kong.response.exit(500, { message = "An authentication error occurred" })
  end

  if res.status >= 200 and res.status < 300 then
    -- Normalize response headers to lowercase for case-insensitive lookup
    local lower_headers = {}
    for k, v in pairs(res.headers) do
      lower_headers[string.lower(k)] = v
    end

    -- Copy trusted headers from Authentik response to upstream service request
    for _, header_name in ipairs(conf.trust_response_headers or {}) do
      local val = lower_headers[string.lower(header_name)]
      if val then
        kong.service.request.set_header(header_name, val)
      end
    end
    return
  end

  -- Authentication failed/redirect: forward headers and response directly to client
  local response_headers = {}
  for k, v in pairs(res.headers) do
    local lower_k = string.lower(k)
    if lower_k == "location" or lower_k == "set-cookie" or lower_k == "www-authenticate" or lower_k == "content-type" then
      response_headers[k] = v
    end
  end

  return kong.response.exit(res.status, res.body, response_headers)
end

return ForwardAuthHandler
