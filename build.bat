@echo off
REM FollowITup 构建脚本 - Windows
REM 用法: build.bat

echo [1/3] 构建前端...
cd frontend
call npm run build
if %ERRORLEVEL% NEQ 0 (
    echo 前端构建失败!
    exit /b 1
)

echo [2/3] 复制前端产物...
cd ..
rmdir /s /q backend\cmd\server\frontend-dist 2>nul
xcopy /e /i frontend\dist backend\cmd\server\frontend-dist

echo [3/3] 编译 Go 后端...
cd backend
go build -o followitup.exe ./cmd/server/
if %ERRORLEVEL% NEQ 0 (
    echo 后端编译失败!
    exit /b 1
)

echo.
echo ========================================
echo  构建完成: backend\followitup.exe
echo  运行: backend\followitup.exe config.yaml
echo ========================================
cd ..

pause
