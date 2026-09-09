package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func testAccServiceConfigDomainsSteps(idx int, params ...any) string {
	var testSteps = []string{
		// Step 0: One domain.
		// Params: resourceName, rndName, certId, origin1host, origin2host, domain0
		`
locals {
	origin_1_id = "origin-1"
	origin_2_id = "origin-2"
}

resource "ioriver_service" "%s" {
	name        = "%s"
	certificates = ["%s"]
	description = "desc"
	config = {
		origins = [
			{
				name = local.origin_1_id
				custom_origin = {
					host     = "%s"
					protocol = "https"
				}
			},
			{
				name = local.origin_2_id
				custom_origin = {
					host     = "%s"
					protocol = "https"
				}
			}
		]
		domains = [
			{
				domain = "%s"
				mappings = [{ target_mapping = local.origin_1_id }]
			},
		]
	}
}`,

		// Step 1: Two domains, both mapped to origin_1.
		// Params: resourceName, rndName, certId, origin1host, origin2host, domain0, domain1
		`
locals {
	origin_1_id = "origin-1"
	origin_2_id = "origin-2"
}

resource "ioriver_service" "%s" {
	name        = "%s"
	certificates = ["%s"]
	description = "desc"
	config = {
		origins = [
			{
				name = local.origin_1_id
				custom_origin = {
					host     = "%s"
					protocol = "https"
				}
			},
			{
				name = local.origin_2_id
				custom_origin = {
					host     = "%s"
					protocol = "https"
				}
			}
		]
		domains = [
			{
				domain = "%s"
				mappings = [{ target_mapping = local.origin_2_id }]
			},
			{
				domain = "%s"
				mappings = [{ target_mapping = local.origin_1_id }]
			}
		]
	}
}`,

		// Step 2: Three domains, each mapped to origin_1.
		// Params: resourceName, rndName, certId, origin1host, origin2host, domain0, domain1, domain2
		`
locals {
	origin_1_id = "origin-1"
	origin_2_id = "origin-2"
}

resource "ioriver_service" "%s" {
	name        = "%s"
	certificates = ["%s"]
	description = "desc"
	config = {
		origins = [
			{
				name = local.origin_1_id
				custom_origin = {
					host     = "%s"
					protocol = "https"
				}
			},
			{
				name = local.origin_2_id
				custom_origin = {
					host     = "%s"
					protocol = "https"
				}
			}
		]
		domains = [
			{
				domain = "%s"
				mappings = [{ target_mapping = local.origin_1_id }]
			},
			{
				domain = "%s"
				mappings = [{ target_mapping = local.origin_1_id }]
			},
			{
				domain = "%s"
				mappings = [{ target_mapping = local.origin_1_id }]
			}
		]
	}
}`,
	}

	if idx < 0 || idx >= len(testSteps) {
		panic("invalid config step index")
	}
	return fmt.Sprintf(testSteps[idx], params...)
}

// testAccServiceConfigDomainNeglected omits the `domain` attribute entirely
// (Optional+Computed): the backend must assign a domain and the provider
// must not crash reconciling a null domain into state.
// A blank certId omits the `certificates` attribute — required for the
// built-in domain flow, which auto-assigns its own internal certificate.
func testAccServiceConfigDomainNeglected(resourceName string, certId string, originHost string) string {
	certsBlock := ""
	if certId != "" {
		certsBlock = fmt.Sprintf(`certificates = ["%s"]`, certId)
	}
	return fmt.Sprintf(`
resource "ioriver_service" "%s" {
	name        = "%s"
	%s
	description = "desc"
	config = {
		origins = [
			{
				name = "origin-1"
				custom_origin = {
					host     = "%s"
					protocol = "https"
				}
			},
		]
		domains = [
			{
				mappings = [{ target_mapping = "origin-1" }]
			},
		]
	}
}`, resourceName, resourceName, certsBlock, originHost)
}

func testAccServiceConfigMultiMappingSteps(idx int, resourceName string, certId string, domainHost string) string {
	var steps = []string{
		// Step 0: 2 origin sets + 2 standalone origins, domain maps to all 4.
		`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificates = ["%s"]
	description = "multi-mapping acceptance test"
	config = {

		origins = [
			{
				name = "origin-a"
				custom_origin = {
					host     = "origin-a.example.com"
					protocol = "https"
				}
			},
			{
				name = "origin-b"
				custom_origin = {
					host     = "origin-b.example.com"
					protocol = "https"
				}
			},
		]
		origin_sets = [
			{
				name = "set-alpha"
				origins = [
					{
						custom_origin = {
							host     = "alpha-primary.example.com"
							protocol = "https"
						}
					},
					{
						custom_origin = {
							host     = "alpha-failover.example.com"
							protocol = "https"
						}
					},
				]
			},
			{
				name = "set-beta"
				origins = [
					{
						custom_origin = {
							host     = "beta-primary.example.com"
							protocol = "https"
						}
					},
					{
						custom_origin = {
							host     = "beta-failover.example.com"
							protocol = "https"
						}
					},
				]
			},
		]
		domains = [
			{
				domain = "%s"
				mappings = [
					{
						target_type    = "origin_set"
						target_mapping = "set-alpha"
						path_pattern   = "/api/*"
					},
					{
						target_type    = "origin_set"
						target_mapping = "set-beta"
						path_pattern   = "/static/*"
					},
					{
						target_mapping = "origin-a"
						path_pattern   = "/images/*"
					},
					{
						target_mapping = "origin-b"
						path_pattern   = "/*"
					},
				]
			}
		]
	}
}`,

		// Step 1: trim to 1 origin set + 1 standalone origin (2 mappings total).
		`
resource "ioriver_service" "%s" {
	name        = "%s"
	certificates = ["%s"]
	description = "multi-mapping acceptance test"
	config = {

		origins = [
			{
				name = "origin-a"
				custom_origin = {
					host     = "origin-a.example.com"
					protocol = "https"
				}
			},
		]
		origin_sets = [
			{
				name = "set-alpha"
				origins = [
					{
						custom_origin = {
							host     = "alpha-primary.example.com"
							protocol = "https"
						}
					},
					{
						custom_origin = {
							host     = "alpha-failover.example.com"
							protocol = "https"
						}
					},
				]
			},
		]
		domains = [
			{
				domain = "%s"
				mappings = [
					{
						target_type    = "origin_set"
						target_mapping = "set-alpha"
						path_pattern   = "/api/*"
					},
					{
						target_mapping = "origin-a"
						path_pattern   = "/*"
					},
				]
			}
		]
	}
}`,
	}

	return fmt.Sprintf(steps[idx], resourceName, resourceName, certId, domainHost)
}

func TestDomainModel_ModelToMap_PreservesProvidedUUID(t *testing.T) {
	t.Parallel()

	domain := &DomainModel{
		UUId:    types.StringValue("11111111-1111-1111-1111-111111111111"),
		Domain:  types.StringValue("www.example.com"),
		Aliases: types.ListValueMust(types.StringType, []attr.Value{}),
		Mappings: types.ListValueMust(
			types.ObjectType{AttrTypes: DomainMappingAttrTypes()},
			[]attr.Value{},
		),
	}

	got, err := domain.ModelToMap(context.Background(), map[string]string{}, map[string]string{})
	if err != nil {
		t.Fatalf("ModelToMap() error = %v", err)
	}

	if got["uuid"] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected provided uuid to be preserved, got %v", got["uuid"])
	}
}

func TestDomainModel_ModelToMap_GeneratesUUIDWhenMissing(t *testing.T) {
	t.Parallel()

	domain := &DomainModel{
		UUId:    types.StringNull(),
		Domain:  types.StringValue("www.example.com"),
		Aliases: types.ListValueMust(types.StringType, []attr.Value{}),
		Mappings: types.ListValueMust(
			types.ObjectType{AttrTypes: DomainMappingAttrTypes()},
			[]attr.Value{},
		),
	}

	got, err := domain.ModelToMap(context.Background(), map[string]string{}, map[string]string{})
	if err != nil {
		t.Fatalf("ModelToMap() error = %v", err)
	}

	uuid, ok := got["uuid"].(string)
	if !ok || uuid == "" {
		t.Fatalf("expected uuid to be generated and sent, got %v", got["uuid"])
	}
	if domain.UUId.ValueString() != uuid {
		t.Fatalf("expected model uuid %q to match sent uuid %q", domain.UUId.ValueString(), uuid)
	}
}

func TestDomainsFromMap_SwapInsertDeleteSingleTransaction(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	transformCtx := &ServiceTransformContext{
		DesiredDomainOrder: []string{"uuid-c", "uuid-new", "uuid-a"},
	}

	// Simulate backend order after one apply that swapped existing domains,
	// inserted a new one, and removed another one.
	domainsArray := []interface{}{
		map[string]interface{}{
			"uuid":     "uuid-a",
			"domain":   "a.example.com",
			"aliases":  []interface{}{},
			"mappings": []interface{}{},
		},
		map[string]interface{}{
			"uuid":     "uuid-c",
			"domain":   "c.example.com",
			"aliases":  []interface{}{},
			"mappings": []interface{}{},
		},
		map[string]interface{}{
			"uuid":     "uuid-new",
			"domain":   "new.example.com",
			"aliases":  []interface{}{},
			"mappings": []interface{}{},
		},
	}

	got, err := DomainsFromMap(ctx, domainsArray, transformCtx, map[string]string{})
	if err != nil {
		t.Fatalf("DomainsFromMap() error = %v", err)
	}

	if len(*got) != 3 {
		t.Fatalf("expected 3 domains, got %d", len(*got))
	}

	ordered := []string{(*got)[0].Domain.ValueString(), (*got)[1].Domain.ValueString(), (*got)[2].Domain.ValueString()}
	expected := []string{"c.example.com", "new.example.com", "a.example.com"}
	for i := range expected {
		if ordered[i] != expected[i] {
			t.Fatalf("unexpected order at index %d: got %q want %q", i, ordered[i], expected[i])
		}
	}
}

func TestDomainModel_ModelToMap_RenameWithSameUUID(t *testing.T) {
	t.Parallel()

	domain := &DomainModel{
		UUId:    types.StringValue("11111111-1111-1111-1111-111111111111"),
		Domain:  types.StringValue("renamed.example.com"),
		Aliases: types.ListValueMust(types.StringType, []attr.Value{}),
		Mappings: types.ListValueMust(
			types.ObjectType{AttrTypes: DomainMappingAttrTypes()},
			[]attr.Value{},
		),
	}

	got, err := domain.ModelToMap(context.Background(), map[string]string{}, map[string]string{})
	if err != nil {
		t.Fatalf("ModelToMap() error = %v", err)
	}

	if got["uuid"] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected uuid to stay the same on rename, got %v", got["uuid"])
	}
	if got["domain"] != "renamed.example.com" {
		t.Fatalf("expected renamed domain to be sent, got %v", got["domain"])
	}
}

func TestDomainsFromMap_ConcurrentDrift_AppendsUnknownDomainsDeterministically(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	transformCtx := &ServiceTransformContext{
		DesiredDomainOrder: []string{"uuid-a", "uuid-b"},
	}

	// Simulate external drift between plan and apply: backend now includes
	// out-of-band domains not present in planned desired order.
	domainsArray := []interface{}{
		map[string]interface{}{
			"uuid":     "uuid-b",
			"domain":   "b.example.com",
			"aliases":  []interface{}{},
			"mappings": []interface{}{},
		},
		map[string]interface{}{
			"uuid":     "uuid-z",
			"domain":   "z.example.com",
			"aliases":  []interface{}{},
			"mappings": []interface{}{},
		},
		map[string]interface{}{
			"uuid":     "uuid-a",
			"domain":   "a.example.com",
			"aliases":  []interface{}{},
			"mappings": []interface{}{},
		},
		map[string]interface{}{
			"uuid":     "uuid-m",
			"domain":   "m.example.com",
			"aliases":  []interface{}{},
			"mappings": []interface{}{},
		},
	}

	got, err := DomainsFromMap(ctx, domainsArray, transformCtx, map[string]string{})
	if err != nil {
		t.Fatalf("DomainsFromMap() error = %v", err)
	}

	ordered := make([]string, 0, len(*got))
	for _, d := range *got {
		ordered = append(ordered, d.Domain.ValueString())
	}

	// Desired names first, then out-of-band domains in stable lexical order.
	expected := []string{"a.example.com", "b.example.com", "m.example.com", "z.example.com"}
	if len(ordered) != len(expected) {
		t.Fatalf("unexpected domain count: got %d want %d", len(ordered), len(expected))
	}
	for i := range expected {
		if ordered[i] != expected[i] {
			t.Fatalf("unexpected order at index %d: got %q want %q", i, ordered[i], expected[i])
		}
	}
}
