package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func GenerateUUID() string {
	uuid := uuid.New()
	return uuid.String()
}

// ListElementsAsDiags decodes a Terraform list value into a typed Go slice and
// returns framework diagnostics so callers can attach path-aware errors.
// Null/unknown lists decode to an empty slice.
func ListElementsAsDiags[T any](ctx context.Context, list types.List) ([]T, diag.Diagnostics) {
	if list.IsNull() || list.IsUnknown() {
		return []T{}, nil
	}
	items := []T{}
	diags := list.ElementsAs(ctx, &items, false)
	return items, diags
}

// ListObjectValueFromDiags encodes a typed Go slice into a Terraform
// list(object) value and returns framework diagnostics.
func ListObjectValueFromDiags[T any](ctx context.Context, attrTypes map[string]attr.Type, items []T) (types.List, diag.Diagnostics) {
	list, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: attrTypes}, items)
	if diags.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: attrTypes}), diags
	}
	return list, diags
}

// ListElementsAs decodes a Terraform list value into a typed Go slice.
// Null/unknown lists decode to an empty slice.
func ListElementsAs[T any](ctx context.Context, list types.List) ([]T, error) {
	items, diags := ListElementsAsDiags[T](ctx, list)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to decode list: %v", diags.Errors())
	}
	return items, nil
}

// ListObjectValueFrom encodes a typed Go slice into a Terraform list(object) value.
func ListObjectValueFrom[T any](ctx context.Context, attrTypes map[string]attr.Type, items []T) (types.List, error) {
	list, diags := ListObjectValueFromDiags(ctx, attrTypes, items)
	if diags.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: attrTypes}), fmt.Errorf("failed to build list: %v", diags.Errors())
	}
	return list, nil
}

// Nameable is an interface for types that have a Name field.
type Nameable interface {
	GetName() string
}

func alignItems[T Nameable](items []T, stateItemsOrder []string) []T {
	// Create a map for quick lookup of items by name
	itemMap := make(map[string]T)
	for _, item := range items {
		itemMap[item.GetName()] = item
	}

	visited := make(map[string]bool)

	// Add items in the order they appear in the State/Plan
	var alignedItems []T
	for _, name := range stateItemsOrder {
		if item, found := itemMap[name]; found {
			alignedItems = append(alignedItems, item)
			visited[name] = true
		}
	}

	// Append any NEW items found in the API that weren't in the State
	// (This handles 'terraform import' or out-of-band additions)
	otherItems := []T{}
	for _, item := range items {
		if !visited[item.GetName()] {
			otherItems = append(otherItems, item)
		}
	}
	sort.SliceStable(otherItems, func(i, j int) bool {
		return otherItems[i].GetName() < otherItems[j].GetName()
	})

	return append(alignedItems, otherItems...)
}
