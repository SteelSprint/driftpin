package main

import (
	"fmt"
	"sort"
)

// D! id=sup_model range-start
type Supplier struct {
	Name          string
	Contact       string
	ItemsSupplied []string
}
// D! id=sup_model range-end

var suppliers = make(map[string]*Supplier)

// D! id=sup_add range-start
func AddSupplier(sup *Supplier) error {
	if len(sup.ItemsSupplied) == 0 {
		return ValidationError("itemsSupplied", "must not be empty")
	}
	if _, exists := suppliers[sup.Name]; exists {
		return ValidationError("name", "duplicate supplier: "+sup.Name)
	}
	suppliers[sup.Name] = sup
	return nil
}
// D! id=sup_add range-end

// D! id=sup_list range-start
func ListSuppliers() []*Supplier {
	out := make([]*Supplier, 0, len(suppliers))
	for _, sup := range suppliers {
		out = append(out, sup)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	for _, sup := range out {
		sort.Strings(sup.ItemsSupplied)
	}
	return out
}
// D! id=sup_list range-end

// D! id=sup_get range-start
func GetSupplier(name string) (*Supplier, error) {
	sup, ok := suppliers[name]
	if !ok {
		return nil, NotFound("supplier", name)
	}
	return sup, nil
}
// D! id=sup_get range-end

var _ = fmt.Sprintf
