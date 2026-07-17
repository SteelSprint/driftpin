package main

import "fmt"

// D! id=err_nf range-start
func NotFound(entityType, key string) error {
	return fmt.Errorf("%s not found: %s", entityType, key)
}
// D! id=err_nf range-end

// D! id=err_ve range-start
func ValidationError(field, reason string) error {
	return fmt.Errorf("invalid %s: %s", field, reason)
}
// D! id=err_ve range-end
