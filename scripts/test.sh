#!/bin/bash

# Test runner script for GoHarness
# This script runs all tests in the project

set -e

echo "🧪 Running GoHarness Test Suite"
echo "=================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print test results
print_result() {
    local test_name="$1"
    local result="$2"
    local duration="$3"
    
    if [ "$result" = "PASS" ]; then
        echo -e "${GREEN}✓ $test_name${NC} - $result (${duration}s)"
    else
        echo -e "${RED}✗ $test_name${NC} - $result (${duration}s)"
    fi
}

# Function to run tests with timeout
run_test_with_timeout() {
    local test_name="$1"
    local test_command="$2"
    local timeout_seconds="${3:-30}"
    
    echo -e "${YELLOW}Running $test_name...${NC}"
    
    # Run test with timeout
    if timeout "$timeout_seconds" bash -c "$test_command"; then
        print_result "$test_name" "PASS" "1"
        return 0
    else
        local exit_code=$?
        if [ $exit_code -eq 124 ]; then
            print_result "$test_name" "TIMEOUT" "timeout"
        else
            print_result "$test_name" "FAIL" "1"
        fi
        return 1
    fi
}

# Check if we're in the right directory
if [ ! -f "go.mod" ]; then
    echo -e "${RED}Error: go.mod not found. Please run this script from the project root.${NC}"
    exit 1
fi

# Install dependencies
echo "📦 Installing dependencies..."
go mod tidy
cd frontend/terminal && npm install && cd ../..

# Run tests
echo ""
echo "🔬 Running Unit Tests"
echo "---------------------"

# Unit tests
run_test_with_timeout "Unit Tests - API" "go test -v ./tests/unit/api/..." 60
run_test_with_timeout "Unit Tests - Config" "go test -v ./tests/unit/config/..." 60

echo ""
echo "🔬 Running Integration Tests"
echo "-----------------------------"

# Integration tests
run_test_with_timeout "Integration Tests - Engine" "go test -v ./tests/integration/..." 120

echo ""
echo "🔬 Running End-to-End Tests"
echo "---------------------------"

# End-to-end tests
run_test_with_timeout "E2E Tests - CLI" "go test -v ./tests/e2e/..." 180

echo ""
echo "🔬 Running Main Application Tests"
echo "---------------------------------"

# Main application tests
run_test_with_timeout "Main Application" "go test -v ./cmd/goharness/..." 60

echo ""
echo "🔬 Running Internal Package Tests"
echo "---------------------------------"

# Internal package tests
run_test_with_timeout "Internal API" "go test -v ./internal/api/..." 60
run_test_with_timeout "Internal Engine" "go test -v ./internal/engine/..." 60
run_test_with_timeout "Internal Tools" "go test -v ./internal/tools/..." 60
run_test_with_timeout "Internal UI" "go test -v ./internal/ui/..." 60

echo ""
echo "📊 Test Summary"
echo "================"

# Generate test coverage report
echo "📈 Generating test coverage report..."
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

echo ""
echo "✅ All tests completed!"
echo "📄 Coverage report generated: coverage.html"

# Check if all tests passed
if [ $? -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed!${NC}"
    exit 1
fi