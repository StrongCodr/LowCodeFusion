#!/bin/bash

# Build script for LowCodeFusion
# Compiles the code and renames the output to "lcf"

set -e

# ANSI color codes
GREEN="\033[0;32m"
BLUE="\033[0;34m"
NC="\033[0m"  # No Color

echo -e "${GREEN}Building LowCodeFusion...${NC}"

# Determine OS for correct executable extension
EXT=""
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]]; then
    EXT=".exe"
fi

# Build the binary with a temporary name
go build -o lowcodefusion$EXT

# Rename the binary to lcf
mv lowcodefusion$EXT lcf$EXT

echo -e "${GREEN}Build complete! Binary is named 'lcf$EXT'${NC}"
echo -e "${GREEN}Run './lcf$EXT' to use the application${NC}"

# —————————————————————————————————————————————
# System-wide completion installation helper:

echo -e "${BLUE}To install system-wide bash completion, run:${NC}"
echo -e "${BLUE}  sudo sh -c './lcf completion bash > /etc/bash_completion.d/lcf'${NC}"
echo -e "${BLUE}Then either restart your shell or run:${NC}"
echo -e "${BLUE}  source /etc/bash_completion.d/lcf${NC}"
