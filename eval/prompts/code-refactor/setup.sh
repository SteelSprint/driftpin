#!/bin/bash
set -e

./drift init
./drift link convert_func main.convert
./drift todo
