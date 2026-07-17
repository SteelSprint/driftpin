package main

// D! id=val_item range-start
func ValidateItem(item Item) error {
	if err := ValidateName(item.Name); err != nil {
		return err
	}
	if err := ValidateQuantity(item.Quantity); err != nil {
		return err
	}
	if err := ValidatePrice(item.Price); err != nil {
		return err
	}
	return nil
}
// D! id=val_item range-end

// D! id=val_name range-start
func ValidateName(name string) error {
	if name == "" {
		return ValidationError("name", "must not be empty")
	}
	if len(name) > 100 {
		return ValidationError("name", "must not exceed 100 characters")
	}
	return nil
}
// D! id=val_name range-end

// D! id=val_qty range-start
func ValidateQuantity(qty int) error {
	if qty <= 0 {
		return ValidationError("quantity", "must be greater than zero")
	}
	return nil
}
// D! id=val_qty range-end

// D! id=val_price range-start
func ValidatePrice(price float64) error {
	if price < 0 {
		return ValidationError("price", "must not be negative")
	}
	return nil
}
// D! id=val_price range-end
