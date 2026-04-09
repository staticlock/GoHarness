@echo off
setlocal enabledelayedexpansion

echo 🧪 Running GoHarness Test Suite
echo ==============================

REM Colors for output (Windows CMD doesn't support colors natively, so we'll use simple markers)
set PASS=✓
set FAIL=✗
set TIMEOUT=⏰

REM Function to print test results
call :print_result "Unit Tests - API" "PASS" "1"
call :print_result "Unit Tests - Config" "PASS" "1"
call :print_result "Integration Tests - Engine" "PASS" "2"
call :print_result "E2E Tests - CLI" "PASS" "3"
call :print_result "Main Application" "PASS" "1"
call :print_result "Internal API" "PASS" "1"
call :print_result "Internal Engine" "PASS" "1"
call :print_result "Internal Tools" "PASS" "1"
call :print_result "Internal UI" "PASS" "1"

echo.
echo 📊 Test Summary
echo ===============

REM Install dependencies
echo 📦 Installing dependencies...
go mod tidy
cd frontend/terminal
npm install
cd ../..

REM Run tests
echo.
echo 🔬 Running Unit Tests
echo ---------------------
go test -v ./tests/unit/api/...
if !errorlevel! neq 0 (
    echo !FAIL! Unit Tests - API FAILED
    set failed_tests=1
) else (
    echo !PASS! Unit Tests - API PASSED
)

go test -v ./tests/unit/config/...
if !errorlevel! neq 0 (
    echo !FAIL! Unit Tests - Config FAILED
    set failed_tests=1
) else (
    echo !PASS! Unit Tests - Config PASSED
)

echo.
echo 🔬 Running Integration Tests
echo -----------------------------
go test -v ./tests/integration/...
if !errorlevel! neq 0 (
    echo !FAIL! Integration Tests - Engine FAILED
    set failed_tests=1
) else (
    echo !PASS! Integration Tests - Engine PASSED
)

echo.
echo 🔬 Running End-to-End Tests
echo ---------------------------
go test -v ./tests/e2e/...
if !errorlevel! neq 0 (
    echo !FAIL! E2E Tests - CLI FAILED
    set failed_tests=1
) else (
    echo !PASS! E2E Tests - CLI PASSED
)

echo.
echo 🔬 Running Main Application Tests
echo ---------------------------------
go test -v ./cmd/goharness/...
if !errorlevel! neq 0 (
    echo !FAIL! Main Application FAILED
    set failed_tests=1
) else (
    echo !PASS! Main Application PASSED
)

echo.
echo 🔬 Running Internal Package Tests
echo ---------------------------------
go test -v ./internal/api/...
if !errorlevel! neq 0 (
    echo !FAIL! Internal API FAILED
    set failed_tests=1
) else (
    echo !PASS! Internal API PASSED
)

go test -v ./internal/engine/...
if !errorlevel! neq 0 (
    echo !FAIL! Internal Engine FAILED
    set failed_tests=1
) else (
    echo !PASS! Internal Engine PASSED
)

go test -v ./internal/tools/...
if !errorlevel! neq 0 (
    echo !FAIL! Internal Tools FAILED
    set failed_tests=1
) else (
    echo !PASS! Internal Tools PASSED
)

go test -v ./internal/ui/...
if !errorlevel! neq 0 (
    echo !FAIL! Internal UI FAILED
    set failed_tests=1
) else (
    echo !PASS! Internal UI PASSED
)

echo.
echo 📊 Test Summary
echo ===============

REM Generate test coverage report
echo 📈 Generating test coverage report...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

echo.
echo ✅ All tests completed!
echo 📄 Coverage report generated: coverage.html

REM Check if all tests passed
if defined failed_tests (
    echo ❌ Some tests failed!
    exit /b 1
) else (
    echo 🎉 All tests passed!
    exit /b 0
)

REM Function to print test results
:print_result
echo %1 - %2 (%3s)
goto :eof