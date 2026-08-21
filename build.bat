@echo off
REM FollowITup build script - Windows
REM Usage: build.bat
REM NOTE: keep this file pure ASCII to avoid codepage issues
cd /d "%~dp0"

echo [1/3] Building frontend...
cd frontend
call npm run build
if %ERRORLEVEL% NEQ 0 (
    echo FRONTEND BUILD FAILED!
    exit /b 1
)

echo [2/3] Copying frontend assets...
cd ..
rmdir /s /q backend\cmd\server\frontend-dist 2>nul
xcopy /e /i frontend\dist backend\cmd\server\frontend-dist

echo [3/3] Compiling Go backend...
cd backend
go build -o followitup.exe ./cmd/server/
if %ERRORLEVEL% NEQ 0 (
    echo BACKEND BUILD FAILED!
    exit /b 1
)

echo.
echo ========================================
echo  BUILD OK: backend\followitup.exe
echo  Run: backend\followitup.exe config.yaml
echo ========================================
cd ..

pause
