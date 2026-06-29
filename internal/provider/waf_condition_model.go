package provider

import (
	"github.com/hashicorp/go-set"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var wafConditionFields = []string{
	"http.request.header",
	"http.request.method",
	"http.request.uri_raw",
	"http.request.path",
	"http.request.json_param",
	"http.request.query_param",
	"http.request.cookie",
	"http.request.body",
	"client.ip.address",
	"client.ip.asn",
	"client.geo.country",
	"action_token.score",
	"bot_validation.result",
	"client.ja3",
	"client.ja4",
}

var wafConditionOperators = []string{
	"eq", "ne", "in", "not_in",
	"contains", "not_contains",
	"regex", "not_regex",
	"begins_with", "not_begins_with",
	"ends_with", "not_ends_with",
	"contains_word", "not_contains_word",
	"ip_match", "not_ip_match",
	"exists", "does_not_exist",
	"lt", "le", "gt", "ge",
}

// wafStringOps is the set of operators valid on free-form string fields
// (body / header / cookie / query_param / json_param value side).
var wafStringOps = []string{
	"eq", "ne", "in", "not_in",
	"contains", "not_contains",
	"regex", "not_regex",
	"begins_with", "not_begins_with",
	"ends_with", "not_ends_with",
	"contains_word", "not_contains_word",
}

// wafCollectionOps = wafStringOps + exists/does_not_exist (key may be absent).
var wafCollectionOps = append(append([]string{}, wafStringOps...), "exists", "does_not_exist")

var WafConditionSpec = &ConditionSpec{
	Name: "waf",
	Operators: map[string]OperatorSpec{
		"eq":                {Arity: arityScalar},
		"ne":                {Arity: arityScalar},
		"in":                {Arity: arityList},
		"not_in":            {Arity: arityList},
		"contains":          {Arity: arityScalar},
		"not_contains":      {Arity: arityScalar},
		"regex":             {Arity: arityScalar},
		"not_regex":         {Arity: arityScalar},
		"begins_with":       {Arity: arityScalar},
		"not_begins_with":   {Arity: arityScalar},
		"ends_with":         {Arity: arityScalar},
		"not_ends_with":     {Arity: arityScalar},
		"contains_word":     {Arity: arityScalar},
		"not_contains_word": {Arity: arityScalar},
		"exists":            {Arity: arityNone},
		"does_not_exist":    {Arity: arityNone},
		"ip_match":          {Arity: arityList, CommaString: true},
		"not_ip_match":      {Arity: arityList, CommaString: true},
		"lt":                {Arity: arityScalar},
		"le":                {Arity: arityScalar},
		"gt":                {Arity: arityScalar},
		"ge":                {Arity: arityScalar},
	},
	Fields: map[string]FieldSpec{
		"http.request.method":      {Kind: kindString, Operators: *set.From[string]([]string{"eq", "ne", "in", "not_in"})},
		"http.request.path":        {Kind: kindPath, Operators: *set.From[string]([]string{"eq", "ne", "in", "not_in", "regex", "not_regex", "contains", "not_contains", "begins_with", "not_begins_with", "ends_with", "not_ends_with", "contains_word", "not_contains_word"})},
		"http.request.uri_raw":     {Kind: kindURL, Operators: *set.From[string]([]string{"eq", "ne", "in", "not_in", "regex", "not_regex", "begins_with", "not_begins_with", "contains", "not_contains", "ends_with", "not_ends_with", "contains_word", "not_contains_word"})},
		"http.request.body":        {Kind: kindString, Operators: *set.From[string](append(append([]string{}, wafStringOps...), "exists", "does_not_exist"))},
		"http.request.header":      {Kind: kindString, RequiresFieldKey: true, Operators: *set.From[string](wafCollectionOps)},
		"http.request.cookie":      {Kind: kindString, RequiresFieldKey: true, Operators: *set.From[string](wafCollectionOps)},
		"http.request.query_param": {Kind: kindString, RequiresFieldKey: true, Operators: *set.From[string](wafCollectionOps)},
		"http.request.json_param":  {Kind: kindString, RequiresFieldKey: true, Operators: *set.From[string](wafCollectionOps)},
		"client.ip.address":        {Kind: kindIP, Operators: *set.From[string]([]string{"ip_match", "not_ip_match"})},
		"client.ip.asn":            {Kind: kindInt, Operators: *set.From[string]([]string{"eq", "ne", "in", "not_in", "lt", "le", "gt", "ge"})},
		"client.geo.country":       {Kind: kindCountry, Operators: *set.From[string]([]string{"eq", "ne", "in", "not_in"})},
		"action_token.score":       {Kind: kindFloat, RequiresFieldKey: true, NumericRange: &range01, Operators: *set.From[string]([]string{"lt", "le", "gt", "ge", "eq", "ne"})},
		"bot_validation.result":    {Kind: kindPassFail, Operators: *set.From[string]([]string{"eq", "ne"})},
		"client.ja3":               {Kind: kindString, Operators: *set.From[string]([]string{"eq", "ne", "in", "not_in"})},
		"client.ja4":               {Kind: kindString, Operators: *set.From[string]([]string{"eq", "ne", "in", "not_in"})},
	},
}

func wafConditionAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"field": schema.StringAttribute{
			MarkdownDescription: "Request field to evaluate.\n" +
				"  - Plain fields (no `field_key` needed):\n" +
				"    - `http.request.method` (HTTP verb)\n" +
				"    - `http.request.path` (URL path, e.g. `/api/users`)\n" +
				"    - `http.request.uri_raw` (full raw URI including query string)\n" +
				"    - `http.request.body` (request body text)\n" +
				"    - `client.ip.address` (client IPv4/IPv6 — use `ip_match`/`not_ip_match`)\n" +
				"    - `client.ip.asn` (autonomous system number)\n" +
				"    - `client.geo.country` (ISO 3166-1 alpha-2 country code)\n" +
				"    - `bot_validation.result` (bot validation boolean result — supports `eq`/`ne`)\n" +
				"    - `client.ja3` (JA3 TLS fingerprint hash)\n" +
				"    - `client.ja4` (JA4 TLS fingerprint hash)\n" +
				"  - Collection fields (require `field_key`):\n" +
				"    - `http.request.header`\n" +
				"    - `http.request.cookie`\n" +
				"    - `http.request.query_param`\n" +
				"    - `http.request.json_param`\n" +
				"    - `action_token.score` (action token score in range 0.0-1.0 — `field_key` selects the token type, supports `lt`/`le`/`gt`/`ge`) \n  -",
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(wafConditionFields...),
			},
		},
		"operator": schema.StringAttribute{
			MarkdownDescription: "Match operator to apply.\n" +
				"  - **String:**\n " +
				"    - `eq` (exact match),\n" +
				"    - `ne` (not equal),\n" +
				"    - `begins_with` / `not_begins_with`,\n" +
				"    - `ends_with` / `not_ends_with`,\n" +
				"    - `contains` / `not_contains` (substring),\n" +
				"    - `contains_word` / `not_contains_word` (word-boundary match),\n" +
				"    - `regex` / `not_regex` (Python-compatible regular expression).\n" +
				"  - **List:**\n" +
				"    - `in` / `not_in` (value is in the supplied list).\n" +
				"  - **IP/CIDR:**\n" +
				"    - `ip_match` / `not_ip_match` (use with `client.ip.address`; supply one or more CIDRs/IPs in `value`).\n" +
				"  - **Existence** (set `value = []`):\n" +
				"    - `exists` / `does_not_exist` (field/header/cookie/param is present or absent).\n" +
				"  - **Numeric** (use with `action_token.score`):\n" +
				"    - `lt`, `le`, `gt`, `ge`. \n  -",
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(wafConditionOperators...),
			},
		},
		"values": conditionValuesAttr(),
		"value":  conditionValueAttr(),
		"field_key": schema.StringAttribute{
			MarkdownDescription: "Name of the specific header, cookie, query parameter, or JSON body field to inspect.\n" +
				"  - **Required** when `field` is one of `http.request.header`, `http.request.cookie`, `http.request.query_param`, or `http.request.json_param`.\n" +
				"  - For example: `field_key = \"User-Agent\"` when `field = \"http.request.header\"`. \n  -",
			Optional: true,
		},
	}
}

func wafConditionExpressionAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"or": schema.ListNestedAttribute{
			MarkdownDescription: "OR-of-ANDs match expression.\n" +
				"  - The rule matches when **at least one** OR group matches.\n" +
				"  - Each OR group contains one or more `and` conditions that must **all** match simultaneously. \n  -",
			Required: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"and": schema.ListNestedAttribute{
						MarkdownDescription: "List of conditions that must ALL match for this OR group to be satisfied.",
						Required:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: wafConditionAttributes(),
						},
					},
				},
			},
			Validators: []validator.List{
				ConditionExpressionValidator(WafConditionSpec),
			},
		},
	}
}
