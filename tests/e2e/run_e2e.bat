@echo off
REM ============================================================================
REM Edge Video V2 - End-to-End Test Suite Runner (Windows)
REM ============================================================================

echo.
echo ╔════════════════════════════════════════════════════════════════════════╗
echo ║                                                                        ║
echo ║   Edge Video V2 - End-to-End Test Suite                              ║
echo ║   Phase 3: Real Camera Testing                                        ║
echo ║                                                                        ║
echo ╚════════════════════════════════════════════════════════════════════════╝
echo.

REM Check if running from v2 directory
if not exist "config.yaml" (
    echo ❌ ERROR: config.yaml not found
    echo    Please run this script from the v2/ directory
    echo.
    pause
    exit /b 1
)

REM Check if producer binary exists
if not exist "bin\producer.exe" (
    echo ⚠️  Producer binary not found. Building...
    go build -o bin\producer.exe .\cmd\producer
    if errorlevel 1 (
        echo ❌ Build failed!
        pause
        exit /b 1
    )
    echo ✅ Build complete
    echo.
)

REM Create results directory
if not exist "tests\e2e\results" mkdir "tests\e2e\results"

REM Generate timestamp
for /f "tokens=2-4 delims=/ " %%a in ('date /t') do (set mydate=%%c%%a%%b)
for /f "tokens=1-2 delims=/: " %%a in ('time /t') do (set mytime=%%a%%b)
set timestamp=%mydate%_%mytime%

REM Set result file
set RESULT_FILE=tests\e2e\results\e2e_results_%timestamp%.txt

echo 📊 Test results will be saved to: %RESULT_FILE%
echo.

REM Start logging
echo Edge Video V2 - E2E Test Results > %RESULT_FILE%
echo Date: %date% %time% >> %RESULT_FILE%
echo. >> %RESULT_FILE%

echo ═══════════════════════════════════════════════════════════════
echo STEP 1: Environment Setup Validation
echo ═══════════════════════════════════════════════════════════════
echo.

go test -v ./tests/e2e -run TestEnvironmentSetup -timeout 5m 2>&1 | tee -a %RESULT_FILE%

if errorlevel 1 (
    echo.
    echo ❌ Environment setup validation FAILED!
    echo    Please fix the issues above before continuing.
    echo.
    pause
    exit /b 1
)

echo.
echo ✅ Environment setup validation PASSED
echo.
pause

echo ═══════════════════════════════════════════════════════════════
echo STEP 2: Single Camera Tests
echo ═══════════════════════════════════════════════════════════════
echo.
echo Running single camera tests (cam1 RTMP + cam2 RTSP)...
echo This will take approximately 2 minutes.
echo.

go test -v ./tests/e2e -run "TestSingleCamera" -timeout 10m 2>&1 | tee -a %RESULT_FILE%

echo.
echo ═══════════════════════════════════════════════════════════════
echo STEP 3: Stress Test (5 Cameras)
echo ═══════════════════════════════════════════════════════════════
echo.
echo ⚠️  WARNING: This test will run for 5 minutes!
echo.
set /p continue="Continue with stress test? (y/n): "
if /i not "%continue%"=="y" (
    echo Skipping stress test.
    goto :shutdown
)

echo.
echo Running 5-camera stress test...
echo.

go test -v ./tests/e2e -run TestStressFiveCameras -timeout 10m 2>&1 | tee -a %RESULT_FILE%

:shutdown
echo.
echo ═══════════════════════════════════════════════════════════════
echo STEP 4: Graceful Shutdown Test
echo ═══════════════════════════════════════════════════════════════
echo.

go test -v ./tests/e2e -run "TestGracefulShutdown$" -timeout 5m 2>&1 | tee -a %RESULT_FILE%

echo.
echo ═══════════════════════════════════════════════════════════════
echo STEP 5: Optional - Soak Test (10 minutes)
echo ═══════════════════════════════════════════════════════════════
echo.
set /p soak="Run 10-minute soak test? (y/n): "
if /i "%soak%"=="y" (
    echo.
    echo Running 10-minute soak test...
    echo.
    go test -v ./tests/e2e -run TestSoakTenMinutes -timeout 15m 2>&1 | tee -a %RESULT_FILE%
) else (
    echo Skipping soak test.
)

echo.
echo ═══════════════════════════════════════════════════════════════
echo 🎉 E2E TEST SUITE COMPLETE
echo ═══════════════════════════════════════════════════════════════
echo.
echo Results saved to: %RESULT_FILE%
echo.
echo Review the results above to ensure all tests passed.
echo.
pause
