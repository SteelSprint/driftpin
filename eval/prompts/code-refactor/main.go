package main

import "fmt"

// D! id=convert_func
func Convert(temp float64, from string, to string) float64 {
	celsius := temp
	switch from {
	case "F":
		celsius = (temp - 32) * 5 / 9
	case "K":
		celsius = temp - 273.15
	case "C":
		celsius = temp
	}

	result := celsius
	switch to {
	case "F":
		result = celsius*9/5 + 32
	case "K":
		result = celsius + 273.15
	case "C":
		result = celsius
	}

	return result
}

func main() {
	fmt.Println("32F to C:", Convert(32, "F", "C"))
	fmt.Println("100C to F:", Convert(100, "C", "F"))
	fmt.Println("0C to K:", Convert(0, "C", "K"))
	fmt.Println("273.15K to C:", Convert(273.15, "K", "C"))
	fmt.Println("212F to K:", Convert(212, "F", "K"))
}
