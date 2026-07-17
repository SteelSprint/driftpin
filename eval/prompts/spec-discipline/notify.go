package main

import (
	"fmt"
	"time"
)

// D! id=notif_notify range-start
func Notify(itemName string, quantity int) string {
	return fmt.Sprintf("[STOCK] %s: %d remaining", itemName, quantity)
}
// D! id=notif_notify range-end

// D! id=notif_alert range-start
func SendAlert(message string) {
	fmt.Println(message)
}
// D! id=notif_alert range-end

// D! id=notif_format range-start
func FormatNotification(message string) string {
	ts := time.Now().UTC().Format(time.RFC3339)
	return fmt.Sprintf("%s [INFO] %s", ts, message)
}
// D! id=notif_format range-end

// D! id=audit_log range-start
var auditEntries []string

func AuditLog(action, entity, details string) {
	ts := time.Now().UTC().Format(time.RFC3339)
	entry := fmt.Sprintf("%s %s %s: %s", ts, action, entity, details)
	auditEntries = append(auditEntries, entry)
}
// D! id=audit_log range-end

// D! id=audit_query range-start
func AuditQuery(action string) []string {
	var out []string
	for _, entry := range auditEntries {
		if action == "" || containsAction(entry, action) {
			out = append(out, entry)
		}
	}
	return out
}

func containsAction(entry, action string) bool {
	return len(entry) > len(action) && entry[len(action)+21:len(action)+21+len(action)] == action
}
// D! id=audit_query range-end
