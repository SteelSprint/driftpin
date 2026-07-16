#!/bin/bash
set -e

./drift init
./drift link add_func main.add
./drift link sub_func main.sub
./drift link mul_func main.mul
./drift link div_func main.div
./drift link main_func main.cli
./drift todo
