package main

import (
	"fmt"
	"sort"
	"strings"
)

// D! id=rep_summary range-start
func Summary(store *Store) string {
	items := store.ListItems()
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Total items: %d\n", len(items)))
	totalValue := 0.0
	totalQty := 0
	for _, item := range items {
		lineValue := float64(item.Quantity) * item.Price
		totalValue += lineValue
		totalQty += item.Quantity
	}
	avgPrice := 0.0
	if len(items) > 0 {
		avgPrice = totalValue / float64(totalQty)
	}
	sb.WriteString(fmt.Sprintf("Total value: %.2f\n", totalValue))
	sb.WriteString(fmt.Sprintf("Average price: %.2f\n", avgPrice))
	sb.WriteString("Name                          Qty    Price    LineValue\n")
	for _, item := range items {
		lineValue := float64(item.Quantity) * item.Price
		sb.WriteString(fmt.Sprintf("%-30s %3d %8.2f %10.2f\n", item.Name, item.Quantity, item.Price, lineValue))
	}
	return sb.String()
}
// D! id=rep_summary range-end

// D! id=rep_lowstock range-start
func LowStockReport(store *Store, threshold int) string {
	items := store.ListItems()
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Low Stock Report (threshold: %d)\n", threshold))
	for _, item := range items {
		if item.Quantity < threshold {
			sb.WriteString(fmt.Sprintf("  %s: %d remaining\n", item.Name, item.Quantity))
			Notify(item.Name, item.Quantity)
		}
	}
	return sb.String()
}
// D! id=rep_lowstock range-end

// D! id=rep_value range-start
func ValueReport(store *Store) string {
	items := store.ListItems()
	type valueEntry struct {
		item  Item
		value float64
	}
	entries := make([]valueEntry, len(items))
	for i, item := range items {
		entries[i] = valueEntry{item: item, value: float64(item.Quantity) * item.Price}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].value > entries[j].value
	})
	var sb strings.Builder
	sb.WriteString("Value Report (sorted by value descending)\n")
	sb.WriteString("Name                          Qty    Price    Value\n")
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("%-30s %3d %8.2f %8.2f\n", e.item.Name, e.item.Quantity, e.item.Price, e.value))
	}
	return sb.String()
}
// D! id=rep_value range-end

// D! id=rep_supplier range-start
func SupplierReport() string {
	sups := ListSuppliers()
	var sb strings.Builder
	sb.WriteString("Supplier Report\n")
	for _, sup := range sups {
		sb.WriteString(fmt.Sprintf("\n%s (Contact: %s)\n", sup.Name, sup.Contact))
		for _, item := range sup.ItemsSupplied {
			sb.WriteString(fmt.Sprintf("  - %s\n", item))
		}
	}
	return sb.String()
}
// D! id=rep_supplier range-end
