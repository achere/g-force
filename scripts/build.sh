#!/bin/bash

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    key="$1"

    case $key in
        --version)
            VERSION="$2"
            shift
            ;;
        *)
            # Unknown option
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
    shift
done

# Check if version is provided
if [ -z "$VERSION" ]; then
    echo "Version not provided. Please provide a version using --version."
    exit 1
fi

# If directory bin doesn't exist, create it
if [ ! -d "bin" ]; then
    mkdir "bin"
    echo "Folder created at bin"
else
    echo "Folder already exists at bin"
fi

# Build binaries
GOOS=darwin GOARCH=arm64 go build -o bin/apexcov-$VERSION-darwin-arm64 ./cmd/apexcov/main.go
GOOS=linux GOARCH=arm64 go build -o bin/apexcov-$VERSION-linux-arm64 ./cmd/apexcov/main.go
GOOS=linux GOARCH=amd64 go build -o bin/apexcov-$VERSION-linux-amd64 ./cmd/apexcov/main.go

# Gzip binaries
gzip -c bin/apexcov-$VERSION-darwin-arm64 > bin/apexcov-$VERSION-darwin-arm64.gz
gzip -c bin/apexcov-$VERSION-linux-arm64 > bin/apexcov-$VERSION-linux-arm64.gz
gzip -c bin/apexcov-$VERSION-linux-amd64 > bin/apexcov-$VERSION-linux-amd64.gz

# Delete all files in the bin directory excluding those with ".gz" extension
cd bin
find . -type f ! -name "*.gz" -exec rm {} +
