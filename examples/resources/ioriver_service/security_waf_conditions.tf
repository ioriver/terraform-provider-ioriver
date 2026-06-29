# Full WAF reference example — every custom_rule action, every rate_limit action,
# every operator, and every field type, each with the exact value format required.
#
# ┌─────────────────────────────────────────────────────────────────────────────┐
# │ CONDITION STRUCTURE                                                          │
# │                                                                              │
# │  condition = {                                                               │
# │    or = [           # at least one OR group must match                      │
# │      {                                                                       │
# │        and = [      # ALL clauses in this group must match                  │
# │          {                                                                   │
# │            field     = "..."   # required                                   │
# │            operator  = "..."   # required                                   │
# │            value     = [...]   # required; use [] for exists/does_not_exist │
# │            field_key = "..."   # required only for collection fields        │
# │          }                                                                   │
# │        ]                                                                     │
# │      }                                                                       │
# │    ]                                                                         │
# │  }                                                                           │
# └─────────────────────────────────────────────────────────────────────────────┘
#
# ┌── FIELDS ──────────────────────────────────────────────────────────────────┐
# │ Plain fields (no field_key):                                                │
# │   http.request.method      — HTTP verb, e.g. "GET"                         │
# │   http.request.path        — URL path, e.g. "/api/users"                   │
# │   http.request.uri_raw     — full raw URI (path + query string)            │
# │   http.request.body        — raw request body text                         │
# │   client.ip.address        — client IPv4/v6; use CIDR with ip_match        │
# │   client.ip.asn            — ASN as string, e.g. "64496"                   │
# │   client.geo.country       — ISO 3166-1 alpha-2 code, e.g. "US"            │
# │                                                                             │
# │                                                                             │
# │ Collection fields (field_key = "<name>" required):                         │
# │   http.request.header      — field_key = header name                       │
# │   http.request.cookie      — field_key = cookie name                       │
# │   http.request.query_param — field_key = query parameter name              │
# │   http.request.json_param  — field_key = JSON body key name                │
# │   action_token.score       — score 0.0–1.0; field_key selects token scope  │
# └─────────────────────────────────────────────────────────────────────────────┘
#
# ┌── OPERATORS ────────────────────────────────────────────────────────────────┐
# │ String matching  (value = ["string"])                                        │
# │   eq               exact match                                              │
# │   ne               not equal                                                │
# │   begins_with      prefix match                                             │
# │   not_begins_with  does NOT start with                                      │
# │   ends_with        suffix match                                             │
# │   not_ends_with    does NOT end with                                        │
# │   contains         substring match                                          │
# │   not_contains     does NOT contain substring                               │
# │   contains_word    whole-word match (word-boundary aware)                   │
# │   not_contains_word  does NOT match as whole word                           │
# │   regex            Python-compatible regex                                  │
# │   not_regex        does NOT match regex                                     │
# │                                                                             │
# │ List membership    (value = ["a", "b", ...])                                │
# │   in               value is in the list                                     │
# │   not_in           value is NOT in the list                                 │
# │                                                                             │
# │ IP / CIDR          (value = ["10.0.0.0/8", ...])                           │
# │   ip_match         client IP falls within any given CIDR                   │
# │   not_ip_match     client IP does NOT fall within any CIDR                 │
# │                                                                             │
# │ Existence          (value = []  — empty list, no value needed)              │
# │   exists           field / header / cookie / param is present               │
# │   does_not_exist   field / header / cookie / param is absent                │
# │                                                                             │
# │ Numeric            (value = ["<number>"]  — numeric string)                 │
# │   lt               less than                                                │
# │   le               less than or equal                                       │
# │   gt               greater than                                             │
# │   ge               greater than or equal                                    │
# │   (only valid for action_token.score)                                       │
# └─────────────────────────────────────────────────────────────────────────────┘
#
# ┌── CUSTOM RULE ACTIONS ──────────────────────────────────────────────────────┐
# │   block                — drop the request, return 403                       │
# │   log                  — allow and record (observe mode)                    │
# │   allow                — explicitly allow, skip remaining rules             │
# │   bypass_managed       — allow and skip the managed checkpoint ruleset      │
# │   challenge            — serve a silent JavaScript challenge                │
# │   interactive_challenge — serve an interactive CAPTCHA challenge            │
# │   ignore               — suppress a WAF flag for a specific parameter       │
# │                          (requires ignore_params block; see below)          │
# └─────────────────────────────────────────────────────────────────────────────┘
#
# ┌── RATE LIMIT ACTIONS ───────────────────────────────────────────────────────┐
# │   block                — drop excess requests                               │
# │   log                  — record excess requests without blocking            │
# │   challenge            — JS challenge for excess requesters                 │
# │   interactive_challenge — CAPTCHA for excess requesters                     │
# └─────────────────────────────────────────────────────────────────────────────┘

resource "ioriver_service" "waf_full" {
  name        = "waf-full"
  certificate = ioriver_certificate.cert.id

  config = {
    origins = [
      {
        name          = "my-origin"
        custom_origin = { host = "origin.example.com", protocol = "https" }
      }
    ]
    domains = [
      {
        domain   = "www.example.com"
        mappings = [{ target_mapping = "my-origin" }]
      }
    ]

    security = {
      enabled = true

      waf = {
        limit_body_size = true

        checkpoint = {
          web_attacks = {
            mode             = "prevent" # learn | prevent | disabled
            confidence_level = "high"    # high | medium | critical
          }
          ips = {
            mode                     = "prevent"
            performance_impact       = "medium" # low | medium | high
            severity                 = "medium" # low | medium | high | critical
            high_confidence_action   = "block"  # block | log
            medium_confidence_action = "block"
            low_confidence_action    = "log"
          }
          trusted_sources     = ["203.0.113.10", "198.51.100.5"]
          minimal_num_sources = 3
        }
      }

      # -----------------------------------------------------------------------
      # SECTION 1 — one rule per CUSTOM RULE ACTION
      # -----------------------------------------------------------------------
      custom_rules = [

        # ── ACTION: allow ─────────────────────────────────────────────────────
        # Explicitly allow trusted internal ranges. Matching requests skip all
        # subsequent rules (including block rules below).
        {
          name    = "allow-internal-ips"
          enabled = true
          action  = "allow"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "client.ip.address"
                    operator = "ip_match"
                    value    = ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]
                  }
                ]
              }
            ]
          }
        },

        # ── ACTION: block ─────────────────────────────────────────────────────
        # Block external access to /admin. Multi-AND: BOTH clauses must match.
        {
          name    = "block-admin-external"
          enabled = true
          action  = "block"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "begins_with"
                    value    = ["/admin"]
                  },
                  {
                    field    = "client.ip.address"
                    operator = "not_ip_match"
                    value    = ["203.0.113.0/24"]
                  }
                ]
              }
            ]
          }
        },

        # ── ACTION: log ───────────────────────────────────────────────────────
        # Observe (but do not block) traffic from high-risk ASNs.
        {
          name    = "log-risky-asn"
          enabled = true
          action  = "log"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "client.ip.asn"
                    operator = "in"
                    value    = ["64496", "64497", "64498"] # ASNs as strings
                  }
                ]
              }
            ]
          }
        },

        # ── ACTION: bypass_managed ────────────────────────────────────────────
        # Skip the managed checkpoint for a trusted partner path whose payloads
        # would otherwise trigger false positives.
        {
          name    = "bypass-partner-webhook"
          enabled = true
          action  = "bypass_managed"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "begins_with"
                    value    = ["/partner/webhook/"]
                  },
                  {
                    field    = "client.ip.address"
                    operator = "ip_match"
                    value    = ["198.51.100.0/24"]
                  }
                ]
              }
            ]
          }
        },

        # ── ACTION: challenge ─────────────────────────────────────────────────
        # Silent JS challenge for clients with a high bot score.
        {
          name    = "challenge-high-bot-score"
          enabled = true
          action  = "challenge"
          condition = {
            or = [
              {
                and = [
                  {
                    field     = "action_token.score"
                    field_key = "web"
                    operator  = "ge"
                    value     = ["0.7"] # numeric string — score 0.7 or above
                  }
                ]
              }
            ]
          }
        },

        # ── ACTION: interactive_challenge ─────────────────────────────────────
        # CAPTCHA for high-risk countries accessing checkout.
        {
          name    = "ichallenge-risky-checkout"
          enabled = true
          action  = "interactive_challenge"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "begins_with"
                    value    = ["/checkout"]
                  },
                  {
                    field    = "client.geo.country"
                    operator = "in"
                    value    = ["CN", "RU", "KP", "BY"] # ISO 3166-1 alpha-2
                  }
                ]
              }
            ]
          }
        },

        # ── ACTION: ignore ────────────────────────────────────────────────────
        # Tell the managed WAF not to flag "token" in the JSON body as malicious
        # for /api/auth requests. The WAF still runs; it just ignores that field.
        #
        # ignore_params.ignore_type options:
        #   json_body_param  — a JSON key in the request body
        #   body_param       — a URL-encoded form field
        #   header           — a request header name
        {
          name    = "ignore-auth-token-field"
          enabled = true
          action  = "ignore"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "begins_with"
                    value    = ["/api/auth"]
                  }
                ]
              }
            ]
          }
          ignore_params = {
            ignore_type = "json_body_param" # json_body_param | body_param | header
            value       = "token"
          }
        },

        # ── Multi-OR / multi-AND compound condition ───────────────────────────
        # Block login abuse via two independent OR groups.
        #   Group 0: POST to /login (path AND method)
        #   Group 1: bad country AND not from a trusted IP
        # Either complete group triggers the block.
        {
          name    = "block-login-abuse"
          enabled = true
          action  = "block"
          condition = {
            or = [
              {
                and = [
                  { field = "http.request.path", operator = "eq", value = ["/login"] },
                  { field = "http.request.method", operator = "eq", value = ["POST"] },
                ]
              },
              {
                and = [
                  { field = "client.geo.country", operator = "in", value = ["KP", "RU"] },
                  { field = "client.ip.address", operator = "not_ip_match", value = ["10.0.0.0/8"] },
                ]
              },
            ]
          }
        },

        # -----------------------------------------------------------------------
        # SECTION 2 — one rule per OPERATOR (showing the exact value format)
        # -----------------------------------------------------------------------

        # ── eq — exact string match ────────────────────────────────────────────
        # value = ["exact string"]
        {
          name    = "op-eq"
          enabled = true
          action  = "block"
          condition = {
            or = [{ and = [{ field = "http.request.path", operator = "eq", value = ["/forbidden"] }] }]
          }
        },

        # ── ne — not equal ────────────────────────────────────────────────────
        # value = ["string"]
        {
          name    = "op-ne"
          enabled = true
          action  = "log"
          condition = {
            or = [{ and = [{ field = "http.request.method", operator = "ne", value = ["GET"] }] }]
          }
        },

        # ── begins_with — prefix ──────────────────────────────────────────────
        # value = ["/prefix"]
        {
          name    = "op-begins-with"
          enabled = true
          action  = "block"
          condition = {
            or = [{ and = [{ field = "http.request.path", operator = "begins_with", value = ["/internal"] }] }]
          }
        },

        # ── not_begins_with ───────────────────────────────────────────────────
        # value = ["/prefix"]
        {
          name    = "op-not-begins-with"
          enabled = true
          action  = "log"
          condition = {
            or = [{ and = [{ field = "http.request.path", operator = "not_begins_with", value = ["/public"] }] }]
          }
        },

        # ── ends_with — suffix ────────────────────────────────────────────────
        # value = [".ext"] — multiple values act as OR within the clause
        {
          name    = "op-ends-with"
          enabled = true
          action  = "block"
          condition = {
            or = [{ and = [{ field = "http.request.path", operator = "ends_with", value = [".php", ".asp", ".aspx"] }] }]
          }
        },

        # ── not_ends_with ─────────────────────────────────────────────────────
        # value = [".ext"]
        {
          name    = "op-not-ends-with"
          enabled = true
          action  = "log"
          condition = {
            or = [{ and = [{ field = "http.request.uri_raw", operator = "not_ends_with", value = [".css", ".js", ".png", ".woff2"] }] }]
          }
        },

        # ── contains — substring ──────────────────────────────────────────────
        # value = ["substring"]
        {
          name    = "op-contains"
          enabled = true
          action  = "block"
          condition = {
            or = [{ and = [{ field = "http.request.body", operator = "contains", value = ["<script>"] }] }]
          }
        },

        # ── not_contains ──────────────────────────────────────────────────────
        # value = ["substring"]
        {
          name    = "op-not-contains"
          enabled = true
          action  = "log"
          condition = {
            or = [{ and = [{ field = "http.request.uri_raw", operator = "not_contains", value = ["utm_source"] }] }]
          }
        },

        # ── contains_word — whole-word match ──────────────────────────────────
        # value = ["word"]  (word-boundary aware, not a simple substring)
        {
          name    = "op-contains-word"
          enabled = true
          action  = "block"
          condition = {
            or = [{ and = [{ field = "http.request.body", operator = "contains_word", value = ["malware"] }] }]
          }
        },

        # ── not_contains_word ─────────────────────────────────────────────────
        # value = ["word"]
        {
          name    = "op-not-contains-word"
          enabled = true
          action  = "log"
          condition = {
            or = [{ and = [{ field = "http.request.body", operator = "not_contains_word", value = ["safe"] }] }]
          }
        },

        # ── regex — Python-compatible regular expression ───────────────────────
        # value = ["pattern"]
        {
          name    = "op-regex"
          enabled = true
          action  = "block"
          condition = {
            or = [{ and = [{ field = "http.request.path", operator = "regex", value = ["/api/v[0-9]+/admin"] }] }]
          }
        },

        # ── not_regex ─────────────────────────────────────────────────────────
        # value = ["pattern"]
        {
          name    = "op-not-regex"
          enabled = true
          action  = "log"
          condition = {
            or = [{ and = [{ field = "http.request.path", operator = "not_regex", value = ["^/static/.*\\.(js|css|png|jpg)$"] }] }]
          }
        },

        # ── in — list membership ──────────────────────────────────────────────
        # value = ["a", "b", ...]  — matches if the field equals any listed value
        {
          name    = "op-in"
          enabled = true
          action  = "block"
          condition = {
            or = [{ and = [{ field = "client.geo.country", operator = "in", value = ["KP", "RU", "BY"] }] }]
          }
        },

        # ── not_in ────────────────────────────────────────────────────────────
        # value = ["a", "b", ...]
        {
          name    = "op-not-in"
          enabled = true
          action  = "block"
          condition = {
            or = [{ and = [{ field = "http.request.method", operator = "not_in", value = ["GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"] }] }]
          }
        },

        # ── ip_match — CIDR range ─────────────────────────────────────────────
        # value = ["cidr", ...]  — only valid for client.ip.address
        {
          name    = "op-ip-match"
          enabled = true
          action  = "allow"
          condition = {
            or = [{ and = [{ field = "client.ip.address", operator = "ip_match", value = ["203.0.113.0/24", "198.51.100.0/24"] }] }]
          }
        },

        # ── not_ip_match ──────────────────────────────────────────────────────
        # value = ["cidr", ...]  — only valid for client.ip.address
        {
          name    = "op-not-ip-match"
          enabled = true
          action  = "challenge"
          condition = {
            or = [{ and = [{ field = "client.ip.address", operator = "not_ip_match", value = ["10.0.0.0/8", "172.16.0.0/12"] }] }]
          }
        },

        # ── exists — field / header / cookie / param is present ───────────────
        # value = []  (empty list — no value is checked, only presence)
        {
          name    = "op-exists"
          enabled = true
          action  = "block"
          condition = {
            or = [{ and = [{
              field     = "http.request.header"
              field_key = "X-Debug-Override" # required for collection fields
              operator  = "exists"
              value     = [] # ← must be empty list
            }] }]
          }
        },

        # ── does_not_exist — field / header / cookie / param is absent ────────
        # value = []  (empty list)
        {
          name    = "op-does-not-exist"
          enabled = true
          action  = "log"
          condition = {
            or = [{ and = [{
              field     = "http.request.cookie"
              field_key = "csrf_token" # required for collection fields
              operator  = "does_not_exist"
              value     = [] # ← must be empty list
            }] }]
          }
        },

        # ── lt / le / gt / ge — numeric comparison ────────────────────────────
        # value = ["<number>"]  (numeric string 0.0-1.0) — only valid for action_token.score
        {
          name    = "op-lt-bot-score"
          enabled = true
          action  = "allow" # very low score = almost certainly human
          condition = {
            or = [{ and = [{ field = "action_token.score", field_key = "web", operator = "lt", value = ["0.1"] }] }]
          }
        },
        {
          name    = "op-le-bot-score"
          enabled = true
          action  = "log"
          condition = {
            or = [{ and = [{ field = "action_token.score", field_key = "web", operator = "le", value = ["0.4"] }] }]
          }
        },
        {
          name    = "op-gt-bot-score"
          enabled = true
          action  = "interactive_challenge"
          condition = {
            or = [{ and = [{ field = "action_token.score", field_key = "web", operator = "gt", value = ["0.6"] }] }]
          }
        },
        {
          name    = "op-ge-bot-score"
          enabled = true
          action  = "block"
          condition = {
            or = [{ and = [{ field = "action_token.score", field_key = "web", operator = "ge", value = ["0.9"] }] }]
          }
        },

        # -----------------------------------------------------------------------
        # SECTION 3 — one rule per COLLECTION FIELD TYPE
        # (field_key is required for all four)
        # -----------------------------------------------------------------------

        # ── http.request.header ───────────────────────────────────────────────
        # field_key = the header name (case-insensitive)
        {
          name    = "field-header"
          enabled = true
          action  = "block"
          condition = {
            or = [{ and = [{
              field     = "http.request.header"
              field_key = "User-Agent" # ← header name
              operator  = "contains"
              value     = ["sqlmap", "nikto", "nmap"]
            }] }]
          }
        },

        # ── http.request.cookie ───────────────────────────────────────────────
        # field_key = the cookie name
        {
          name    = "field-cookie"
          enabled = true
          action  = "block"
          condition = {
            or = [{ and = [{
              field     = "http.request.cookie"
              field_key = "session" # ← cookie name
              operator  = "regex"
              value     = ["[<>'\"\\\\]"] # malformed session value
            }] }]
          }
        },

        # ── http.request.query_param ──────────────────────────────────────────
        # field_key = the URL query parameter name
        {
          name    = "field-query-param"
          enabled = true
          action  = "block"
          condition = {
            or = [{ and = [{
              field     = "http.request.query_param"
              field_key = "id" # ← query param name
              operator  = "regex"
              value     = ["(?i)(union|select|insert|drop|delete|update)"]
            }] }]
          }
        },

        # ── http.request.json_param ───────────────────────────────────────────
        # field_key = the JSON body key name
        {
          name    = "field-json-param"
          enabled = true
          action  = "block"
          condition = {
            or = [{ and = [{
              field     = "http.request.json_param"
              field_key = "filename" # ← JSON key name
              operator  = "ends_with"
              value     = [".exe", ".bat", ".sh", ".ps1"]
            }] }]
          }
        },

      ] # end custom_rules

      # -----------------------------------------------------------------------
      # SECTION 4 — one rate_limit rule per RATE LIMIT ACTION
      # -----------------------------------------------------------------------
      rate_limit = [

        # ── ACTION: block ─────────────────────────────────────────────────────
        # Drop excess requests. Hard limit on the login endpoint:
        # 5 requests per 10 s → blocked for 5 minutes.
        {
          name                   = "rl-login-block"
          enabled                = true
          action                 = "block" # block | log | challenge | interactive_challenge
          num_of_requests        = 5
          time_window_seconds    = 10
          block_duration_seconds = 300
          condition = {
            or = [
              {
                and = [
                  { field = "http.request.path", operator = "eq", value = ["/login"] }
                ]
              }
            ]
          }
        },

        # ── ACTION: log ───────────────────────────────────────────────────────
        # Record excess requests without blocking. Observe API volume:
        # 1000 req / 60 s.
        {
          name                   = "rl-api-log"
          enabled                = true
          action                 = "log"
          num_of_requests        = 1000
          time_window_seconds    = 60
          block_duration_seconds = 60
          condition = {
            or = [
              {
                and = [
                  { field = "http.request.path", operator = "begins_with", value = ["/api/"] }
                ]
              }
            ]
          }
        },

        # ── ACTION: challenge ─────────────────────────────────────────────────
        # JS challenge for clients hammering search from high-risk countries:
        # 30 req / 10 s.
        {
          name                   = "rl-search-challenge"
          enabled                = true
          action                 = "challenge"
          num_of_requests        = 30
          time_window_seconds    = 10
          block_duration_seconds = 120
          condition = {
            or = [
              {
                and = [
                  { field = "http.request.path", operator = "begins_with", value = ["/search"] },
                  { field = "client.geo.country", operator = "in", value = ["RU", "CN", "KP"] },
                ]
              }
            ]
          }
        },

        # ── ACTION: interactive_challenge ─────────────────────────────────────
        # CAPTCHA for checkout abuse from non-internal IPs: 20 req / 10 s →
        # soft-blocked for 10 minutes.
        {
          name                   = "rl-checkout-interactive"
          enabled                = true
          action                 = "interactive_challenge"
          num_of_requests        = 20
          time_window_seconds    = 10
          block_duration_seconds = 600
          condition = {
            or = [
              {
                and = [
                  { field = "http.request.path", operator = "begins_with", value = ["/checkout"] },
                  { field = "client.ip.address", operator = "not_ip_match", value = ["10.0.0.0/8"] },
                ]
              }
            ]
          }
        },

      ] # end rate_limit

    } # end security
  }
}

#
# CONDITION STRUCTURE
#   condition = {
#     or = [           # at least one OR group must match
#       {
#         and = [      # ALL clauses in this group must match
#           { field = "...", operator = "...", value = [...] }
#         ]
#       }
#     ]
#   }
#
# PLAIN FIELDS (no field_key)
#   http.request.method      http.request.path     http.request.uri_raw
#   http.request.body        client.ip.address     client.ip.asn
#   client.geo.country
#
# COLLECTION FIELDS (field_key = "<name>" required)
#   http.request.header      http.request.cookie
#   http.request.query_param http.request.json_param  action_token.score
#
# OPERATORS
#   String:  eq  ne  begins_with  not_begins_with  ends_with  not_ends_with
#            contains  not_contains  contains_word  not_contains_word
#            regex  not_regex
#   List:    in  not_in
#   IP/CIDR: ip_match  not_ip_match
#   Exists:  exists  does_not_exist  (use value = [])
#   Numeric: lt  le  gt  ge  (for action_token.score, value 0.0-1.0)
#
# CUSTOM RULE ACTIONS
#   block  log  allow  bypass_managed  challenge  interactive_challenge  ignore
#
# RATE LIMIT ACTIONS
#   block  log  challenge  interactive_challenge

resource "ioriver_service" "waf_full" {
  name        = "waf-full"
  certificate = ioriver_certificate.cert.id

  config = {
    origins = [
      {
        name          = "my-origin"
        custom_origin = { host = "origin.example.com", protocol = "https" }
      }
    ]
    domains = [
      {
        domain   = "www.example.com"
        mappings = [{ target_mapping = "my-origin" }]
      }
    ]

    security = {
      enabled = true

      waf = {
        limit_body_size = true

        checkpoint = {
          web_attacks = {
            mode             = "prevent"
            confidence_level = "high"
          }
          ips = {
            mode                     = "prevent"
            performance_impact       = "medium"
            severity                 = "medium"
            high_confidence_action   = "block"
            medium_confidence_action = "block"
            low_confidence_action    = "log"
          }
          trusted_sources     = ["203.0.113.10", "198.51.100.5"]
          minimal_num_sources = 3
        }
      }

      # -----------------------------------------------------------------------
      # custom_rules — one entry per action type, in evaluation order.
      # -----------------------------------------------------------------------
      custom_rules = [

        # ── allow ────────────────────────────────────────────────────────────
        # Allow traffic from trusted internal IP ranges before any block rules.
        {
          name    = "allow-internal"
          enabled = true
          action  = "allow"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "client.ip.address"
                    operator = "ip_match"
                    value    = ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]
                  }
                ]
              }
            ]
          }
        },

        # ── block ─────────────────────────────────────────────────────────────
        # Block requests to /admin that are NOT from a known office CIDR.
        # Multi-AND: both clauses must match for the rule to fire.
        {
          name    = "block-admin-external"
          enabled = true
          action  = "block"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "begins_with"
                    value    = ["/admin"]
                  },
                  {
                    field    = "client.ip.address"
                    operator = "not_ip_match"
                    value    = ["203.0.113.0/24"]
                  }
                ]
              }
            ]
          }
        },

        # ── block — collection field (header) ─────────────────────────────────
        # Block requests whose User-Agent contains a known scanner signature.
        {
          name    = "block-scanner-ua"
          enabled = true
          action  = "block"
          condition = {
            or = [
              {
                and = [
                  {
                    field     = "http.request.header"
                    field_key = "User-Agent" # the header name to inspect
                    operator  = "contains"
                    value     = ["sqlmap", "nikto", "nmap"]
                  }
                ]
              }
            ]
          }
        },

        # ── block — collection field (query_param) ────────────────────────────
        # Block SQL injection attempts via a query parameter.
        {
          name    = "block-sqli-query"
          enabled = true
          action  = "block"
          condition = {
            or = [
              {
                and = [
                  {
                    field     = "http.request.query_param"
                    field_key = "id"
                    operator  = "regex"
                    value     = ["(?i)(union|select|insert|drop|delete|update)"]
                  }
                ]
              }
            ]
          }
        },

        # ── block — collection field (json_param) ─────────────────────────────
        # Block when a JSON body field contains a dangerous file extension.
        {
          name    = "block-upload-ext"
          enabled = true
          action  = "block"
          condition = {
            or = [
              {
                and = [
                  {
                    field     = "http.request.json_param"
                    field_key = "filename"
                    operator  = "ends_with"
                    value     = [".exe", ".bat", ".sh", ".ps1"]
                  }
                ]
              }
            ]
          }
        },

        # ── log ───────────────────────────────────────────────────────────────
        # Observe (but do not block) traffic from high-risk ASNs.
        {
          name    = "log-high-risk-asn"
          enabled = true
          action  = "log"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "client.ip.asn"
                    operator = "in"
                    value    = ["64496", "64497", "64498"]
                  }
                ]
              }
            ]
          }
        },

        # ── log — collection field (cookie) ───────────────────────────────────
        # Log requests where the session cookie is missing (unauthenticated).
        {
          name    = "log-no-session-cookie"
          enabled = true
          action  = "log"
          condition = {
            or = [
              {
                and = [
                  {
                    field     = "http.request.cookie"
                    field_key = "session"
                    operator  = "does_not_exist"
                    value     = []
                  }
                ]
              }
            ]
          }
        },

        # ── bypass_managed ───────────────────────────────────────────────────
        # Skip the managed checkpoint ruleset for a trusted partner integration
        # path so its non-standard payloads are not flagged as attacks.
        {
          name    = "bypass-partner-webhook"
          enabled = true
          action  = "bypass_managed"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "begins_with"
                    value    = ["/partner/webhook/"]
                  },
                  {
                    field    = "client.ip.address"
                    operator = "ip_match"
                    value    = ["198.51.100.0/24"]
                  }
                ]
              }
            ]
          }
        },

        # ── challenge ────────────────────────────────────────────────────────
        # Serve a silent JavaScript challenge to clients with elevated bot scores.
        {
          name    = "challenge-high-bot-score"
          enabled = true
          action  = "challenge"
          condition = {
            or = [
              {
                and = [
                  {
                    field     = "action_token.score"
                    field_key = "web"
                    operator  = "ge"
                    value     = ["0.7"]
                  }
                ]
              }
            ]
          }
        },

        # ── interactive_challenge ─────────────────────────────────────────────
        # Require an interactive CAPTCHA for traffic from high-risk countries
        # that targets the checkout flow.
        {
          name    = "ichallenge-risky-country-checkout"
          enabled = true
          action  = "interactive_challenge"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "begins_with"
                    value    = ["/checkout"]
                  },
                  {
                    field    = "client.geo.country"
                    operator = "in"
                    value    = ["CN", "RU", "KP", "BY"]
                  }
                ]
              }
            ]
          }
        },

        # ── ignore ───────────────────────────────────────────────────────────
        # Tell the managed WAF not to flag the "token" JSON field as malicious
        # when the request targets the /api/auth path.
        # ignore_params.ignore_type: json_body_param | body_param | header
        {
          name    = "ignore-auth-token-field"
          enabled = true
          action  = "ignore"
          condition = {
            or = [
              {
                and = [
                  {
                    field    = "http.request.path"
                    operator = "begins_with"
                    value    = ["/api/auth"]
                  }
                ]
              }
            ]
          }
          ignore_params = {
            ignore_type = "json_body_param"
            value       = "token"
          }
        },

        # ── multi-OR / multi-AND compound condition ───────────────────────────
        # Block login abuse:
        #   OR group 0: POST to /login  (path AND method must both match)
        #   OR group 1: known bad country that is NOT from a trusted IP
        # If either group matches in full, the request is blocked.
        {
          name    = "block-login-abuse"
          enabled = true
          action  = "block"
          condition = {
            or = [
              {
                and = [
                  { field = "http.request.path", operator = "eq", value = ["/login"] },
                  { field = "http.request.method", operator = "eq", value = ["POST"] },
                ]
              },
              {
                and = [
                  { field = "client.geo.country", operator = "in", value = ["KP", "RU"] },
                  { field = "client.ip.address", operator = "not_ip_match", value = ["10.0.0.0/8"] },
                ]
              },
            ]
          }
        },

      ] # end custom_rules

      # -----------------------------------------------------------------------
      # rate_limit — one entry per action type.
      # -----------------------------------------------------------------------
      rate_limit = [

        # ── block ─────────────────────────────────────────────────────────────
        # Hard rate limit on the login endpoint: 5 req / 10 s → block for 5 min.
        {
          name                   = "rl-login-block"
          enabled                = true
          action                 = "block"
          num_of_requests        = 5
          time_window_seconds    = 10
          block_duration_seconds = 300
          condition = {
            or = [
              {
                and = [
                  { field = "http.request.path", operator = "eq", value = ["/login"] }
                ]
              }
            ]
          }
        },

        # ── log ───────────────────────────────────────────────────────────────
        # Observe API traffic volume without blocking: 1000 req / 60 s.
        {
          name                   = "rl-api-log"
          enabled                = true
          action                 = "log"
          num_of_requests        = 1000
          time_window_seconds    = 60
          block_duration_seconds = 60
          condition = {
            or = [
              {
                and = [
                  { field = "http.request.path", operator = "begins_with", value = ["/api/"] }
                ]
              }
            ]
          }
        },

        # ── challenge ────────────────────────────────────────────────────────
        # Serve a JS challenge to clients that hammer the search endpoint from
        # high-risk countries: 30 req / 10 s.
        {
          name                   = "rl-search-challenge"
          enabled                = true
          action                 = "challenge"
          num_of_requests        = 30
          time_window_seconds    = 10
          block_duration_seconds = 120
          condition = {
            or = [
              {
                and = [
                  { field = "http.request.path", operator = "begins_with", value = ["/search"] },
                  { field = "client.geo.country", operator = "in", value = ["RU", "CN", "KP"] },
                ]
              }
            ]
          }
        },

        # ── interactive_challenge ─────────────────────────────────────────────
        # CAPTCHA for checkout abuse from non-internal IPs: 20 req / 10 s →
        # block for 10 min.
        {
          name                   = "rl-checkout-interactive"
          enabled                = true
          action                 = "interactive_challenge"
          num_of_requests        = 20
          time_window_seconds    = 10
          block_duration_seconds = 600
          condition = {
            or = [
              {
                and = [
                  { field = "http.request.path", operator = "begins_with", value = ["/checkout"] },
                  { field = "client.ip.address", operator = "not_ip_match", value = ["10.0.0.0/8"] },
                ]
              }
            ]
          }
        },

      ] # end rate_limit

    } # end security
  }
}
