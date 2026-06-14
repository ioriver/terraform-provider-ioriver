package provider

import (
	"sort"

	"github.com/google/uuid"
)

func GenerateUUID() string {
	uuid := uuid.New()
	return uuid.String()
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
