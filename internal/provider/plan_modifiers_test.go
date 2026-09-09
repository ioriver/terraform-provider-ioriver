package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDomainListPlanModifier_SwapDeleteInsert_KeepsIdentityUUIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	elemType := types.ObjectType{AttrTypes: DomainAttrTypes()}

	stateList := types.ListValueMust(elemType, []attr.Value{
		testDomainObj("a.example.com", "uuid-a"),
		testDomainObj("b.example.com", "uuid-b"),
		testDomainObj("c.example.com", "uuid-c"),
	})

	// Simulate: swap (C first), delete B, insert D in the middle.
	// Also simulate stale positional carry-over for new D (uuid-b).
	planList := types.ListValueMust(elemType, []attr.Value{
		testDomainObjWithUUIDUnknown("c.example.com"),
		testDomainObj("d.example.com", "uuid-b"),
		testDomainObjWithUUIDUnknown("a.example.com"),
	})

	modifier := DomainListPlanModifier()
	req := planmodifier.ListRequest{
		PlanValue:  planList,
		StateValue: stateList,
	}
	resp := &planmodifier.ListResponse{PlanValue: planList}

	modifier.PlanModifyList(ctx, req, resp)

	if resp.PlanValue.IsNull() || resp.PlanValue.IsUnknown() {
		t.Fatalf("expected planned domains list, got null/unknown")
	}

	elems := resp.PlanValue.Elements()
	if len(elems) != 3 {
		t.Fatalf("expected 3 planned domains, got %d", len(elems))
	}

	assertDomainUUID(t, elems[0], "c.example.com", "uuid-c", false)
	assertDomainUUID(t, elems[1], "d.example.com", "", true)
	assertDomainUUID(t, elems[2], "a.example.com", "uuid-a", false)
}

func assertDomainUUID(t *testing.T, elem attr.Value, wantDomain, wantUUID string, wantUnknown bool) {
	t.Helper()

	obj, ok := elem.(types.Object)
	if !ok {
		t.Fatalf("expected types.Object element, got %T", elem)
	}

	domain, ok := obj.Attributes()["domain"].(types.String)
	if !ok {
		t.Fatalf("expected domain attribute to be types.String")
	}
	if domain.IsUnknown() || domain.IsNull() || domain.ValueString() != wantDomain {
		t.Fatalf("unexpected domain: got=%q unknown=%v null=%v want=%q", domain.ValueString(), domain.IsUnknown(), domain.IsNull(), wantDomain)
	}

	uuid, ok := obj.Attributes()["uuid"].(types.String)
	if !ok {
		t.Fatalf("expected uuid attribute to be types.String")
	}
	if wantUnknown {
		if !uuid.IsUnknown() {
			t.Fatalf("expected uuid to be unknown for domain %q, got %q", wantDomain, uuid.ValueString())
		}
		return
	}
	if uuid.IsUnknown() || uuid.IsNull() || uuid.ValueString() != wantUUID {
		t.Fatalf("unexpected uuid for domain %q: got=%q unknown=%v null=%v want=%q", wantDomain, uuid.ValueString(), uuid.IsUnknown(), uuid.IsNull(), wantUUID)
	}
}

func testDomainObj(domain, uuid string) attr.Value {
	return types.ObjectValueMust(DomainAttrTypes(), map[string]attr.Value{
		"uuid":     types.StringValue(uuid),
		"domain":   types.StringValue(domain),
		"aliases":  types.ListValueMust(types.StringType, []attr.Value{}),
		"mappings": types.ListValueMust(types.ObjectType{AttrTypes: DomainMappingAttrTypes()}, []attr.Value{}),
	})
}

func testDomainObjWithUUIDUnknown(domain string) attr.Value {
	return types.ObjectValueMust(DomainAttrTypes(), map[string]attr.Value{
		"uuid":     types.StringUnknown(),
		"domain":   types.StringValue(domain),
		"aliases":  types.ListValueMust(types.StringType, []attr.Value{}),
		"mappings": types.ListValueMust(types.ObjectType{AttrTypes: DomainMappingAttrTypes()}, []attr.Value{}),
	})
}
