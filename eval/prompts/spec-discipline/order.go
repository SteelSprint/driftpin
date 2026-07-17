package main

import (
	"fmt"
	"time"
)

// D! id=ord_model range-start
type Order struct {
	ID       string
	ItemName string
	Quantity int
	Status   string
}
// D! id=ord_model range-end

var orders = make(map[string]*Order)

// D! id=ord_create range-start
func CreateOrder(store *Store, itemName string, qty int) (*Order, error) {
	items := store.ListItems()
	for _, item := range items {
		if item.Name == itemName {
			if item.Quantity < qty {
				return nil, ValidationError("quantity", fmt.Sprintf("insufficient stock: have %d, need %d", item.Quantity, qty))
			}
			id := fmt.Sprintf("ORD-%d", time.Now().UnixNano())
			order := &Order{ID: id, ItemName: itemName, Quantity: qty, Status: "pending"}
			orders[id] = order
			AuditLog("create", itemName, fmt.Sprintf("order %s for %d units", id, qty))
			return order, nil
		}
	}
	return nil, NotFound("item", itemName)
}
// D! id=ord_create range-end

// D! id=ord_fulfill range-start
func FulfillOrder(store *Store, orderID string) error {
	order, ok := orders[orderID]
	if !ok {
		return NotFound("order", orderID)
	}
	if order.Status != "pending" {
		return ValidationError("status", fmt.Sprintf("order %s is %s, cannot fulfill", orderID, order.Status))
	}
	if err := store.RemoveItem(order.ItemName); err != nil {
		return err
	}
	shipping := CalculateShipping(order)
	order.Status = "fulfilled"
	SendAlert(Notify(order.ItemName, 0))
	_ = shipping
	AuditLog("fulfill", order.ItemName, fmt.Sprintf("order %s fulfilled, shipping: %.2f", orderID, shipping))
	return nil
}
// D! id=ord_fulfill range-end

// D! id=ord_cancel range-start
func CancelOrder(orderID, reason string) error {
	order, ok := orders[orderID]
	if !ok {
		return NotFound("order", orderID)
	}
	if order.Status == "fulfilled" {
		return ValidationError("status", fmt.Sprintf("order %s is already fulfilled", orderID))
	}
	order.Status = "cancelled"
	AuditLog("cancel", order.ItemName, fmt.Sprintf("order %s cancelled: %s", orderID, reason))
	return nil
}
// D! id=ord_cancel range-end
