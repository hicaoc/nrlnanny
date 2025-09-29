#!/bin/bash
 

echo "Building binaries..."

 make

# ARM64
# GOOS=linux GOARCH=arm64 go build -o bin/nrlnanny_linux_arm64.exe 
# echo "Build complete."