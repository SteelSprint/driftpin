package main

import (
	"fmt"
	"os"
)

// D! id=main_entry range-start
func main() {
	cfgPath := "config.json"
	cfg := LoadConfig(cfgPath)
	store := NewStore()
	store.Load(cfg.DatabasePath)
	args := os.Args[1:]
	Dispatch(store, cfg, args)
	store.Save(cfg.DatabasePath)
}

func init() {
	if len(os.Args) > 1 && os.Args[1] == "--help" {
		fmt.Println("Supply Chain Management System")
		fmt.Println("usage: supplychain [--api-key KEY] [--config PATH] <command> [args]")
		os.Exit(0)
	}
}
// D! id=main_entry range-end
