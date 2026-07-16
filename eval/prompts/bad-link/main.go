package main

import (
	"fmt"
	"strings"
)

// D! id=reverse_func
func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// D! id=palindrome_func
func isPalindrome(s string) bool {
	s = strings.ToLower(s)
	return s == reverse(s)
}

// D! id=wordcount_func
func wordCount(s string) int {
	return len(strings.Fields(s))
}

func main() {
	fmt.Println(reverse("hello"))
	fmt.Println(isPalindrome("racecar"))
	fmt.Println(wordCount("the quick brown fox"))
}
