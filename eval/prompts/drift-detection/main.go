package main

import (
	"fmt"
	"os"
	"strconv"
)

// D! id=add_func
func add(a, b float64) float64 { return a + b }

// D! id=sub_func
func sub(a, b float64) float64 { return a - b }

// D! id=mul_func
func mul(a, b float64) float64 { return a * b }

// D! id=div_func
func div(a, b float64) (float64, error) {
	if b == 0 {
		return 0, fmt.Errorf("division by zero")
	}
	return a / b, nil
}

// D! id=main_func
func main() {
	if len(os.Args) < 4 {
		fmt.Println("usage: calc <a> <op> <b>")
		os.Exit(1)
	}
	a, err := strconv.ParseFloat(os.Args[1], 64)
	if err != nil {
		fmt.Println("invalid a:", err)
		os.Exit(1)
	}
	b, err := strconv.ParseFloat(os.Args[3], 64)
	if err != nil {
		fmt.Println("invalid b:", err)
		os.Exit(1)
	}
	op := os.Args[2]
	switch op {
	case "+":
		fmt.Println(add(a, b))
	case "-":
		fmt.Println(sub(a, b))
	case "*":
		fmt.Println(mul(a, b))
	case "/":
		result, err := div(a, b)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println(result)
	default:
		fmt.Println("unknown operator:", op)
		os.Exit(1)
	}
}
