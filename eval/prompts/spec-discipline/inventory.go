package main

import (
	"sort"
	"strings"
)

// D! id=inv_model range-start
type Item struct {
	Name     string
	Quantity int
	Price    float64
}
// D! id=inv_model range-end

// D! id=inv_add range-start
func (s *Store) AddItem(item Item) error {
	if err := ValidateItem(item); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.items {
		if existing.Name == item.Name {
			return ValidationError("name", "duplicate item: "+item.Name)
		}
	}
	s.items = append(s.items, item)
	return nil
}
// D! id=inv_add range-end

// D! id=inv_remove range-start
func (s *Store) RemoveItem(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.items {
		if item.Name == name {
			s.items = append(s.items[:i], s.items[i+1:]...)
			return nil
		}
	}
	return NotFound("item", name)
}
// D! id=inv_remove range-end

// D! id=inv_list range-start
func (s *Store) ListItems() []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Item, len(s.items))
	copy(out, s.items)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}
// D! id=inv_list range-end

// D! id=srch_items range-start
func (s *Store) SearchItems(query string) []Item {
	if query == "" {
		return s.ListItems()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	query = strings.ToLower(query)
	var out []Item
	for _, item := range s.items {
		if strings.Contains(strings.ToLower(item.Name), query) {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}
// D! id=srch_items range-end

// D! id=srch_filter range-start
func FilterByPrice(items []Item, min, max float64) []Item {
	var out []Item
	for _, item := range items {
		if item.Price >= min && item.Price <= max {
			out = append(out, item)
		}
	}
	return out
}
// D! id=srch_filter range-end
