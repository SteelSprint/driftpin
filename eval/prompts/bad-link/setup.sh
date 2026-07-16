#!/bin/bash
set -e

./drift init
# Correct links for reverse and wordcount
./drift link reverse_func main.reverse
./drift link wordcount_func main.wordcount
# WRONG link: palindrome_func linked to main.reverse instead of main.palindrome
./drift link palindrome_func main.reverse
./drift todo
