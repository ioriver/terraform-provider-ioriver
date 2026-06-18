package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// ─── HCL config generators (called from service_resource_test.go) ─────────────

// testAccCheckServiceConfigWithGeoFencing emits a service resource with an
// explicit geo_fencing block. `countries` is rendered as an HCL list literal —
// Terraform accepts list literals where the schema declares a set, dedup /
// ordering happens implicitly.
func testAccCheckServiceConfigWithGeoFencing(resourceName, certId, mode string, countries []string) string {
	quoted := make([]string, len(countries))
	for i, c := range countries {
		quoted[i] = fmt.Sprintf("%q", c)
	}
	countriesHCL := "[" + strings.Join(quoted, ", ") + "]"

	return fmt.Sprintf(`
resource "%s" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "A generic service"

	config = {
		geo_fencing = {
			mode      = %q
			countries = %s
		}
	}
}`, serviceResourceType, resourceName, resourceName, certId, mode, countriesHCL)
}

// testAccCheckServiceConfigWithoutGeoFencing emits a service resource with the
// geo_fencing block entirely omitted — used to verify the "no default" semantic
// (block goes back to null in state, no diff loop).
func testAccCheckServiceConfigWithoutGeoFencing(resourceName, certId string) string {
	return fmt.Sprintf(`
resource "%s" "%s" {
	name        = "%s"
	certificate = "%s"
	description = "A generic service"

	config = {
	}
}`, serviceResourceType, resourceName, resourceName, certId)
}

// ─── Unit tests ──────────────────────────────────────────────────────────────

// TestGeoFencing_RoundTrip covers the GeoFencing model end-to-end:
//
//  1. ModelToMap on a populated block produces the exact backend wire shape
//     ({"mode": "deny", "countries": [...]}).
//  2. GeoFencingMapToModel on a wire-shaped response rebuilds an equivalent
//     model (sets are order-agnostic, so membership is checked).
//  3. The nil-receiver / nil-input contracts that the parent config_model.go
//     relies on to distinguish "block absent" from "block empty":
//     - (*GeoFencingModel)(nil).ModelToMap(ctx) must return nil so the
//     caller skips the "geo_restriction" wire key entirely (preserving the
//     backend's `optional=True` semantics).
//     - GeoFencingMapToModel(ctx, nil) must return nil so the caller leaves
//     ServiceConfigModel.GeoFencing as nil (no diff vs. HCL that omits
//     the block).
//  4. The smallest-allowed shape — a single-country list — round-trips
//     cleanly. This is the SizeAtLeast(1) boundary; backend rejects empty
//     (HTTP 400), schema rejects empty at plan time.
func TestGeoFencing_RoundTrip(t *testing.T) {
	ctx := context.Background()

	// ── Build the model — mode=deny + 2 countries (happy path). ──────────────
	countriesSet, diags := stringSet([]string{"US", "DE"})
	if diags.HasError() {
		t.Fatalf("stringSet: %v", diags)
	}
	model := &GeoFencingModel{
		Countries: countriesSet,
		Mode:      strVal("deny"),
	}

	// ── ModelToMap: wire shape verification. ─────────────────────────────────
	apiMap := model.ModelToMap(ctx)
	if apiMap == nil {
		t.Fatal("ModelToMap returned nil for a populated model")
	}
	if got := apiMap["mode"]; got != "deny" {
		t.Errorf(`expected mode="deny", got %v`, got)
	}
	gotCountries, ok := apiMap["countries"].([]string)
	if !ok {
		t.Fatalf("expected countries to be []string, got %T", apiMap["countries"])
	}
	if len(gotCountries) != 2 {
		t.Fatalf("expected 2 countries on the wire, got %d (%v)", len(gotCountries), gotCountries)
	}
	// Set order is non-deterministic — check membership.
	wireMembership := map[string]bool{}
	for _, c := range gotCountries {
		wireMembership[c] = true
	}
	for _, want := range []string{"US", "DE"} {
		if !wireMembership[want] {
			t.Errorf("wire countries missing %q, got %v", want, gotCountries)
		}
	}

	// ── GeoFencingMapToModel: simulate the backend response shape ──────────
	// JSON unmarshals lists as []interface{}, not []string — exercise that path.
	wire := map[string]interface{}{
		"mode":      "deny",
		"countries": []interface{}{"US", "DE"},
	}
	recovered := GeoFencingMapToModel(ctx, wire)
	if recovered == nil {
		t.Fatal("GeoFencingMapToModel returned nil for a non-nil map")
	}
	if recovered.Mode.IsNull() || recovered.Mode.ValueString() != "deny" {
		t.Errorf(`expected recovered mode="deny", got %v`, recovered.Mode)
	}
	var recCountries []string
	if d := recovered.Countries.ElementsAs(ctx, &recCountries, false); d.HasError() {
		t.Fatalf("ElementsAs on recovered.Countries: %v", d)
	}
	recMembership := map[string]bool{}
	for _, c := range recCountries {
		recMembership[c] = true
	}
	for _, want := range []string{"US", "DE"} {
		if !recMembership[want] {
			t.Errorf("recovered countries missing %q, got %v", want, recCountries)
		}
	}

	// ── Nil contracts — absent block ≠ empty block. ──────────────────────────
	var nilModel *GeoFencingModel
	if got := nilModel.ModelToMap(ctx); got != nil {
		t.Errorf("ModelToMap on nil receiver must return nil so the parent skips the key; got %v", got)
	}
	if got := GeoFencingMapToModel(ctx, nil); got != nil {
		t.Errorf("GeoFencingMapToModel on nil input must return nil so state stays nil; got %v", got)
	}

	// ── Edge case: smallest allowed shape — single-country allow-list. ───────
	// SizeAtLeast(1) means this is the floor of valid input. Verifies the
	// boundary still serialises correctly.
	singleSet, _ := stringSet([]string{"IL"})
	allowMap := (&GeoFencingModel{Countries: singleSet, Mode: strVal("allow")}).ModelToMap(ctx)
	if allowMap == nil {
		t.Fatal("ModelToMap returned nil for an allow-with-one-country block")
	}
	if got, ok := allowMap["countries"].([]string); !ok || len(got) != 1 || got[0] != "IL" {
		t.Errorf(`expected "countries": ["IL"], got %T %v`, allowMap["countries"], allowMap["countries"])
	}
	if got := allowMap["mode"]; got != "allow" {
		t.Errorf(`expected mode="allow", got %v`, got)
	}
}
