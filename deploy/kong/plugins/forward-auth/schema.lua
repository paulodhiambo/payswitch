local typedefs = require "kong.db.schema.typedefs"

return {
  name = "forward-auth",
  fields = {
    {
      config = {
        type = "record",
        fields = {
          { address = { type = "string", required = true } },
          { http_method = { type = "string", default = "GET", one_of = { "GET", "POST", "HEAD" } } },
          { trust_response_headers = { type = "array", elements = { type = "string" }, default = {} } },
        },
      },
    },
  },
}
