@echo off
REM ============================================================
REM Edge Video V2 - QA Test Suite Executor
REM ============================================================

echo.
echo ============================================================
echo   EDGE VIDEO V2 - QA TEST SUITE
echo ============================================================
echo.

cd /d "%~dp0.."

REM Create reports directory
if not exist "tests\reports" mkdir "tests\reports"

echo [1/5] Running Unit Tests...
echo.
go test ./internal/health/... -v -cover -coverprofile=tests/reports/health_coverage.out
if %errorlevel% neq 0 (
    echo ❌ Health tests FAILED!
    goto :error
)

go test ./internal/resilience/... -v -cover -coverprofile=tests/reports/circuit_breaker_coverage.out
if %errorlevel% neq 0 (
    echo ❌ Circuit Breaker tests FAILED!
    goto :error
)

echo.
echo ✅ Unit tests PASSED!
echo.

echo [2/5] Running Benchmarks...
echo.
go test ./internal/health/... -bench=. -benchmem > tests/reports/health_benchmarks.txt
go test ./internal/resilience/... -bench=. -benchmem > tests/reports/circuit_breaker_benchmarks.txt

echo.
echo ✅ Benchmarks completed!
echo.

echo [3/5] Generating Coverage Reports...
echo.
go tool cover -html=tests/reports/health_coverage.out -o tests/reports/health_coverage.html
go tool cover -html=tests/reports/circuit_breaker_coverage.out -o tests/reports/circuit_breaker_coverage.html

echo ✅ Coverage reports generated!
echo    - tests/reports/health_coverage.html
echo    - tests/reports/circuit_breaker_coverage.html
echo.

echo [4/5] Generating Coverage Summary...
echo.
go tool cover -func=tests/reports/health_coverage.out > tests/reports/health_coverage_summary.txt
go tool cover -func=tests/reports/circuit_breaker_coverage.out > tests/reports/circuit_breaker_coverage_summary.txt

type tests\reports\health_coverage_summary.txt
echo.
type tests\reports\circuit_breaker_coverage_summary.txt

echo.
echo [5/5] Test Summary
echo.
echo ============================================================
echo   TEST RESULTS
echo ============================================================
echo ✅ All unit tests PASSED
echo ✅ Benchmarks completed
echo ✅ Coverage reports generated
echo.
echo Reports available at:
echo   - tests/reports/health_coverage.html
echo   - tests/reports/circuit_breaker_coverage.html
echo   - tests/reports/*_benchmarks.txt
echo ============================================================
echo.

goto :end

:error
echo.
echo ============================================================
echo   ❌ TESTS FAILED!
echo ============================================================
echo.
echo Check output above for details.
echo.
exit /b 1

:end
echo.
echo Press any key to exit...
pause > nul
