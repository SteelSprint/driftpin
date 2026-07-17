package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// D! id=cli_dispatch range-start
func Dispatch(store *Store, cfg Config, args []string) {
	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}
	apiKey := getFlag(args, "--api-key")
	if apiKey == "" || !Authenticate(apiKey) {
		fmt.Fprintln(os.Stderr, "error: authentication required (--api-key)")
		os.Exit(1)
	}
	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "add":
		handleAdd(store, rest)
	case "list":
		handleList(store, rest)
	case "remove":
		handleRemove(store, rest)
	case "order":
		handleOrder(store, rest)
	case "report":
		handleReport(store, cfg, rest)
	case "search":
		handleSearch(store, rest)
	default:
		printUsage()
		os.Exit(1)
	}
}
// D! id=cli_dispatch range-end

// D! id=cli_parser range-start
func ParseArgs(args []string) map[string]string {
	out := make(map[string]string)
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "--") {
			if idx := strings.Index(a, "="); idx >= 0 {
				out[a[2:idx]] = a[idx+1:]
			} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				out[a[2:]] = args[i+1]
				i++
			} else {
				out[a[2:]] = "true"
			}
		}
	}
	return out
}

func getFlag(args []string, flag string) string {
	flags := ParseArgs(args)
	return flags[strings.TrimPrefix(flag, "--")]
}
// D! id=cli_parser range-end

// D! id=cli_add range-start
func handleAdd(store *Store, args []string) {
	flags := ParseArgs(args)
	name := flags["name"]
	qty, _ := strconv.Atoi(flags["quantity"])
	if qty == 0 {
		qty = 1
	}
	price, _ := strconv.ParseFloat(flags["price"], 64)
	item := Item{Name: name, Quantity: qty, Price: price}
	if err := store.AddItem(item); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println("added:", name)
}
// D! id=cli_add range-end

// D! id=cli_list range-start
func handleList(store *Store, args []string) {
	items := store.ListItems()
	fmt.Printf("%-30s %5s %10s\n", "Name", "Qty", "Price")
	for _, item := range items {
		fmt.Printf("%-30s %5d %10.2f\n", item.Name, item.Quantity, item.Price)
	}
}

func handleRemove(store *Store, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: remove <name>")
		os.Exit(1)
	}
	if err := store.RemoveItem(args[0]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println("removed:", args[0])
}

func handleOrder(store *Store, args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: order <item> <qty>")
		os.Exit(1)
	}
	qty, _ := strconv.Atoi(args[1])
	order, err := CreateOrder(store, args[0], qty)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println("order created:", order.ID)
}

func handleReport(store *Store, cfg Config, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: report <summary|lowstock|value|supplier>")
		os.Exit(1)
	}
	switch args[0] {
	case "summary":
		fmt.Println(Summary(store))
	case "lowstock":
		fmt.Println(LowStockReport(store, cfg.LowStockThreshold))
	case "value":
		fmt.Println(ValueReport(store))
	case "supplier":
		fmt.Println(SupplierReport())
	default:
		fmt.Fprintln(os.Stderr, "unknown report:", args[0])
		os.Exit(1)
	}
}

func handleSearch(store *Store, args []string) {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}
	items := store.SearchItems(query)
	fmt.Printf("%-30s %5s %10s\n", "Name", "Qty", "Price")
	for _, item := range items {
		fmt.Printf("%-30s %5d %10.2f\n", item.Name, item.Quantity, item.Price)
	}
}

func printUsage() {
	fmt.Println("usage: supplychain [--api-key KEY] <command> [args]")
	fmt.Println("commands: add, list, remove, order, report, search")
}
// D! id=cli_list range-end
