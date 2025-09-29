#!/bin/bash
# build_windows.sh

echo "Building Windows binaries..."


apt install libasound2-dev


 
# 64-bit
GOOS=windows GOARCH=amd64 go build -o nrlnanny.exe  


# ARM64
# GOOS=linux GOARCH=arm64 go build -o bin/nrlnanny_linux_arm64.exe 
# echo "Build complete."