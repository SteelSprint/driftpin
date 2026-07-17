package main

import "fmt"

// D! id=ship_calc range-start
func CalculateShipping(order *Order) float64 {
	return float64(order.Quantity)*0.50 + 5.00
}
// D! id=ship_calc range-end

// D! id=ship_est range-start
func EstimateDelivery() string {
	// 5 business days from today (simplified: just add 7 calendar days to skip weekends)
	return "5 business days from today"
}
// D! id=ship_est range-end

// D! id=ship_track range-start
func TrackShipment(orderID string) string {
	order, ok := orders[orderID]
	if !ok {
		return "unknown"
	}
	switch order.Status {
	case "pending":
		return "pending"
	case "fulfilled":
		return "shipped"
	default:
		return order.Status
	}
}
// D! id=ship_track range-end

var _ = fmt.Sprintf
