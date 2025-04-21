#!/bin/bash

# Build script for LowCodeFusion
# Compiles the code and renames the output to "lcf"

set -e

echo "Building LowCodeFusion..."

# Determine OS for correct executable extension
EXT=""
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]]; then
    EXT=".exe"
fi

# Build the binary with a temporary name
go build -o lowcodefusion$EXT

# Rename the binary to lcf
mv lowcodefusion$EXT lcf$EXT

echo "Build complete! Binary is named 'lcf$EXT'"
echo "Run './lcf$EXT' to use the application"

