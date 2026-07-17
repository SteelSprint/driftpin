package main

import (
	"encoding/json"
	"os"
	"sync"
)

// D! id=st_model range-start
type Store struct {
	mu    sync.Mutex
	items []Item
}
// D! id=st_model range-end

func NewStore() *Store {
	return &Store{items: make([]Item, 0)}
}

// D! id=st_save_load range-start
func (s *Store) Save(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (s *Store) Load(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.items = make([]Item, 0)
			return nil
		}
		return err
	}
	return s.deserialize(data)
}
// D! id=st_save_load range-end

// D! id=st_ser range-start
func (s *Store) Serialize() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	type itemJSON struct {
		Name     string  `json:"name"`
		Quantity int     `json:"quantity"`
		Price    float64 `json:"price"`
	}
	out := make([]itemJSON, len(s.items))
	for i, item := range s.items {
		out[i] = itemJSON{Name: item.Name, Quantity: item.Quantity, Price: item.Price}
	}
	return json.Marshal(out)
}
// D! id=st_ser range-end

// D! id=st_deser range-start
func (s *Store) deserialize(data []byte) error {
	type itemJSON struct {
		Name     string  `json:"name"`
		Quantity int     `json:"quantity"`
		Price    float64 `json:"price"`
	}
	var items []itemJSON
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	s.items = make([]Item, 0, len(items))
	for _, ij := range items {
		if ij.Name == "" {
			continue
		}
		s.items = append(s.items, Item{Name: ij.Name, Quantity: ij.Quantity, Price: ij.Price})
	}
	return nil
}
// D! id=st_deser range-end
