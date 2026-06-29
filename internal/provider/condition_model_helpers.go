package provider

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/go-set"
)

// What kind of cardinality the operator expects.
type opArity int

const (
	arityScalar opArity = iota // value/values must contain exactly one element
	arityList                  // values must contain >= 1 elements
	arityNone                  // value must be absent
)

// What the element type is on the wire.
type valueKind string

const (
	kindString    valueKind = "string"
	kindInt       valueKind = "integer"
	kindFloat     valueKind = "float"
	kindBool      valueKind = "boolean"
	kindPassFail  valueKind = "pass_fail" // "passed"/"failed" on wire → true/false to backend
	kindPath      valueKind = "path"
	kindIP        valueKind = "ip"         // IP or CIDR
	kindURL       valueKind = "url"        // full http(s):// URL
	kindURLPrefix valueKind = "url_prefix" // prefix of an http(s):// URL
	kindRegex     valueKind = "regex"      // must compile as Go regexp
	kindCountry   valueKind = "country"    // ISO 3166-1 alpha-2 (optional, can start as kindString)
)

// range01 is the [0.0, 1.0] range used by score fields.
var range01 = struct{ Min, Max float64 }{0.0, 1.0}

// One operator entry, *per caller*: tells the engine the arity and any
// element-kind override for this operator on this caller (default = field's kind).
type OperatorSpec struct {
	Arity       opArity
	CommaString bool // backend wants comma-joined string (e.g. WAF client.ip.address)
}

// One field entry, *per caller*: kind, whether it requires field_key,
// the list of operators allowed on it, optional per-(field,operator)
// validators (range, URL, path), and a wire-shape override.
type FieldSpec struct {
	Kind             valueKind
	RequiresFieldKey bool                        // header, cookie, query_param, json_param, ...
	Operators        set.Set[string]             // allowed operators for this field
	NumericRange     *struct{ Min, Max float64 } // e.g. action_token.score → 0..1
}

// The caller (WAF / Behavior) just supplies these two tables + the operator
// catalog. Everything else (validation, ser/deser type coercion) is generic.
type ConditionSpec struct {
	Name      string                  // "waf" / "behavior" — used in error prefixes
	Operators map[string]OperatorSpec // global catalog for this caller
	Fields    map[string]FieldSpec
}

type conditionNativeKind string

const (
	nativeKindInt   conditionNativeKind = "int"
	nativeKindFloat conditionNativeKind = "float"
	nativeKindBool  conditionNativeKind = "bool"
)

var EmptyValueOperators = map[string]bool{
	"exists":         true,
	"does_not_exist": true,
}

type ConditionModelCaller struct {
	ListStyleOperators map[string]bool
	CommaStringField   string
	NativeKinds        map[string]conditionNativeKind
	CollectionFields   map[string]bool
}

func parseConditionNativeValue(raw string, kind valueKind) (interface{}, bool) {
	switch kind {
	case kindInt:
		v, err := strconv.Atoi(raw)
		if err != nil {
			return nil, false
		}
		return v, true
	case kindFloat:
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, false
		}
		return v, true
	case kindBool:
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, false
		}
		return v, true
	case kindPassFail:
		switch raw {
		case "passed":
			return true, true
		case "failed":
			return false, true
		default:
			return nil, false
		}
	default:
		return raw, true
	}
}

func coerceConditionScalar(raw string, kind valueKind) interface{} {
	v, ok := parseConditionNativeValue(raw, kind)
	if !ok {
		return raw
	}
	return v
}

func coerceConditionList(vals []string, kind valueKind) interface{} {
	if kind != kindInt && kind != kindFloat && kind != kindBool && kind != kindPassFail {
		return vals // already []string, marshals as JSON string array
	}

	out := make([]interface{}, 0, len(vals))
	for _, raw := range vals {
		v, _ := parseConditionNativeValue(raw, kind)
		out = append(out, v)
	}
	return out
}

func conditionValuesFromModel(ctx context.Context, cond ConditionModel) []string {
	var vals []string
	if !cond.Values.IsNull() && !cond.Values.IsUnknown() {
		_ = cond.Values.ElementsAs(ctx, &vals, false)
	}
	if len(vals) == 0 && !cond.Value.IsNull() && !cond.Value.IsUnknown() && cond.Value.ValueString() != "" {
		vals = []string{cond.Value.ValueString()}
	}
	return vals
}

func ConditionExpressionToMapByCaller(ctx context.Context, expr *ConditionExpressionModel, spec *ConditionSpec) map[string]interface{} {
	if expr == nil {
		return nil
	}

	orArr := []interface{}{}
	for _, andGroup := range expr.Or {
		andArr := []interface{}{}
		for _, cond := range andGroup.And {
			field := cond.Field.ValueString()
			op := cond.Operator.ValueString()
			vals := conditionValuesFromModel(ctx, cond)

			condMap := map[string]interface{}{
				"field":    field,
				"operator": op,
			}

			fieldSpec := spec.Fields[field]
			opSpec := spec.Operators[op]

			if !cond.FieldKey.IsNull() && !cond.FieldKey.IsUnknown() && cond.FieldKey.ValueString() != "" {
				condMap["field_key"] = cond.FieldKey.ValueString()
			}

			switch {
			case opSpec.Arity == arityNone:
				condMap["value"] = nil
			case opSpec.CommaString:
				condMap["value"] = strings.Join(vals, ",")
			case opSpec.Arity == arityList:
				condMap["value"] = coerceConditionList(vals, fieldSpec.Kind)
			default:
				condMap["value"] = coerceConditionScalar(vals[0], fieldSpec.Kind)
			}

			andArr = append(andArr, condMap)
		}
		orArr = append(orArr, map[string]interface{}{"and": andArr})
	}
	return map[string]interface{}{"or": orArr}
}

func ValidateCondition(ctx context.Context, cond ConditionModel, loc string, spec *ConditionSpec) []string {
	field := cond.Field.ValueString()
	op := cond.Operator.ValueString()
	fk := cond.FieldKey.ValueString()

	// Operator valid.
	opSpec, ok := spec.Operators[op]
	if !ok {
		return []string{fmt.Sprintf("%s: operator %q is not supported by %s conditions", loc, op, spec.Name)}
	}

	// Field valid.
	fieldSpec, ok := spec.Fields[field]
	if !ok {
		return []string{fmt.Sprintf("%s: field %q is not supported by %s conditions", loc, field, spec.Name)}
	}

	// Operator allowed for THIS field
	if ok := fieldSpec.Operators.Contains(op); !ok {
		return []string{fmt.Sprintf(
			"%s: operator %q is not allowed for field %q (allowed: %s)",
			loc, op, field, joinSorted(&fieldSpec.Operators))}
	}

	// field_key exist only if needed for field.
	switch {
	case fieldSpec.RequiresFieldKey && fk == "":
		return []string{fmt.Sprintf("%s: field %q requires field_key to be set", loc, field)}
	case !fieldSpec.RequiresFieldKey && fk != "":
		return []string{fmt.Sprintf("%s: field_key must not be set for field %q", loc, field)}
	}

	// Check (value / values shape) — Check existence, then size, then types.
	hasValue := !cond.Value.IsNull() && !cond.Value.IsUnknown() && cond.Value.ValueString() != ""
	valuesSet := !cond.Values.IsNull() && !cond.Values.IsUnknown()
	var values []string
	if valuesSet {
		_ = cond.Values.ElementsAs(ctx, &values, false)
	}
	if hasValue && len(values) > 0 {
		return []string{fmt.Sprintf("%s: condition %s: cannot set both value and values", loc, op)}
	}

	// Check if has value/values for the operator needs.
	switch opSpec.Arity {
	case arityNone:
		if hasValue || len(values) > 0 {
			return []string{fmt.Sprintf("%s: operator %q must not be given a value", loc, op)}
		}
		return nil // nothing else to check
	case arityScalar:
		// Single value: either `value = "x"` OR `values = ["x"]`. Comma-string fields
		if len(values) > 1 {
			return []string{fmt.Sprintf(
				"%s: operator %q accepts a single value, got %d", loc, op, len(values))}
		}
		fallthrough
	case arityList:
		// Must have value/values set.
		if !hasValue && len(values) == 0 {
			return []string{fmt.Sprintf("%s: operator %q requires a value", loc, op)}
		}
	}

	// Per-element validation. The field's Kind is the single source of truth;
	// operator-specific tweaks (e.g. regex on a path field) live inside
	// validateElement's per-kind switch.
	all := values
	if hasValue {
		all = append(all, cond.Value.ValueString())
	}

	var errs []string
	for _, raw := range all {
		if e := validateElement(raw, fieldSpec.Kind, fieldSpec.NumericRange, loc, field, op); e != "" {
			errs = append(errs, e)
		}
	}
	return errs
}

// ---------------------------------------------------------------------------
// Helpers used by the spec / validator.
// ---------------------------------------------------------------------------

// validateElement parses/validates a single value against the effective kind.
// Returns "" on success, otherwise an error string.
func validateElement(raw string, kind valueKind, rng *struct{ Min, Max float64 }, loc, field, op string) string {
	switch kind {
	case kindString, kindCountry:
		// kindCountry can be tightened to ISO 3166-1 alpha-2 later if desired.
		return ""

	case kindInt:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Sprintf("%s: %s: value %q is not a valid int", loc, field, raw)
		}
		if rng != nil && (float64(n) < rng.Min || float64(n) > rng.Max) {
			return fmt.Sprintf("%s: %s: value %q out of range [%v, %v]", loc, field, raw, rng.Min, rng.Max)
		}

	case kindFloat:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Sprintf("%s: %s: value %q is not a valid float", loc, field, raw)
		}
		if rng != nil && (f < rng.Min || f > rng.Max) {
			return fmt.Sprintf("%s: %s: value %q out of range [%v, %v]", loc, field, raw, rng.Min, rng.Max)
		}

	case kindBool:
		if _, err := strconv.ParseBool(raw); err != nil {
			return fmt.Sprintf("%s: %s: value %q is not a valid bool", loc, field, raw)
		}

	case kindPassFail:
		if raw != "passed" && raw != "failed" {
			return fmt.Sprintf("%s: %s: value %q must be \"passed\" or \"failed\"", loc, field, raw)
		}

	case kindIP:
		if net.ParseIP(raw) == nil {
			if _, _, err := net.ParseCIDR(raw); err != nil {
				return fmt.Sprintf("%s: %s: value %q is not a valid IP address or CIDR", loc, field, raw)
			}
		}

	case kindURL:
		// uri_raw is operator-sensitive:
		// - eq/ne/in/not_in require a full URL
		// - begins_with/not_begins_with require a URL prefix
		// - regex/not_regex require a valid regexp
		// - contains/not_contains/ends_with/not_ends_with/contains_word/not_contains_word are free-form
		switch op {
		case "eq", "ne", "in", "not_in":
			if !isValidFullURL(raw) {
				return fmt.Sprintf("%s: %s + %q requires a full URL (http:// or https://), got %q", loc, field, op, raw)
			}
		case "begins_with", "not_begins_with":
			if !isValidURLPrefix(raw) {
				return fmt.Sprintf("%s: %s + %q requires a valid URL prefix, got %q", loc, field, op, raw)
			}
		case "regex", "not_regex":
			if _, err := regexp.Compile(raw); err != nil {
				return fmt.Sprintf("%s: %s value %q is not a valid regex: %v", loc, field, raw, err)
			}
		}

	case kindURLPrefix:
		if !isValidURLPrefix(raw) {
			return fmt.Sprintf("%s: %s + %q requires a valid URL prefix, got %q", loc, field, op, raw)
		}

	case kindPath:
		// Path rules depend on the operator — keep the existing semantics:
		// eq/ne: must start with '/', no '*', allowed chars, <= 255
		// match/not_match, in/not_in/...: allowed chars, <= 255 (no '*' rule)
		// regex/not_regex: must compile
		switch op {
		case "regex", "not_regex":
			if _, err := regexp.Compile(raw); err != nil {
				return fmt.Sprintf("%s: %s value %q is not a valid regex: %v", loc, field, raw, err)
			}
		default:
			if !strings.HasPrefix(raw, "/") && (op == "eq" || op == "ne" || op == "match" || op == "not_match") {
				return fmt.Sprintf("%s: %s value %q must start with '/'", loc, field, raw)
			}
			if len(raw) > 255 {
				return fmt.Sprintf("%s: %s value %q exceeds 255 chars", loc, field, raw)
			}
			if (op == "eq" || op == "ne") && strings.Contains(raw, "*") {
				return fmt.Sprintf("%s: %s value %q must not contain '*' for operator %q", loc, field, raw, op)
			}
			if !pathAllowedChars.MatchString(raw) {
				return fmt.Sprintf("%s: %s value %q contains invalid characters for operator %q", loc, field, raw, op)
			}
		}

	case kindRegex:
		if _, err := regexp.Compile(raw); err != nil {
			return fmt.Sprintf("%s: %s value %q is not a valid regex: %v", loc, field, raw, err)
		}
	}
	return ""
}

// joinSorted prints an operator set deterministically for error messages.
func joinSorted(s *set.Set[string]) string {
	items := s.Slice()
	sort.Strings(items)
	return strings.Join(items, ", ")
}

func ValidateFieldKeyRules(cond ConditionModel, loc string, collectionFields map[string]bool) []string {
	field := cond.Field.ValueString()
	fieldKey := cond.FieldKey.ValueString()

	if collectionFields[field] {
		if fieldKey == "" {
			return []string{fmt.Sprintf("%s: field '%s' requires field_key to be set", loc, field)}
		}
	} else {
		if fieldKey != "" {
			return []string{fmt.Sprintf("%s: field_key must not be set for non-collection field '%s'", loc, field)}
		}
	}
	return nil
}

// pathAllowedChars mirrors the backend's path_pattern_contains_allowed_chars check.
var pathAllowedChars = regexp.MustCompile(`^/[A-Za-z0-9_\-\.\*\$/~"'\@:\+]*$`)
