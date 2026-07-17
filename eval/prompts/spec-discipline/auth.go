package main

// D! id=auth_check range-start
var validAPIKeys = []string{"key123", "key456", "key789"}

func Authenticate(apiKey string) bool {
	for _, k := range validAPIKeys {
		if k == apiKey {
			return true
		}
	}
	return false
}
// D! id=auth_check range-end

// D! id=authz_check range-start
func Authorize(apiKey, action string) bool {
	return Authenticate(apiKey)
}
// D! id=authz_check range-end
