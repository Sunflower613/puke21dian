@echo off
chcp 65001 >nul
echo 🎰 开始编译21点游戏服务器...
echo.

REM 设置输出目录
set OUTPUT_DIR=.\build
if not exist "%OUTPUT_DIR%" mkdir "%OUTPUT_DIR%"

REM 编译Linux版本
echo 📦 编译Linux版本 (amd64)...
set GOOS=linux
set GOARCH=amd64
go build -o "%OUTPUT_DIR%\blackjack-linux-amd64" .
if %ERRORLEVEL% EQU 0 (
    echo ✅ Linux ^(amd64^) 编译成功: %OUTPUT_DIR%\blackjack-linux-amd64
) else (
    echo ❌ Linux ^(amd64^) 编译失败
    exit /b 1
)

REM 编译Linux ARM64版本
echo 📦 编译Linux版本 ^(arm64^)...
set GOOS=linux
set GOARCH=arm64
go build -o "%OUTPUT_DIR%\blackjack-linux-arm64" .
if %ERRORLEVEL% EQU 0 (
    echo ✅ Linux ^(arm64^) 编译成功: %OUTPUT_DIR%\blackjack-linux-arm64
) else (
    echo ❌ Linux ^(arm64^) 编译失败
    exit /b 1
)

REM 编译Windows版本（用于本地测试）
echo 📦 编译Windows版本 ^(amd64^)...
set GOOS=windows
set GOARCH=amd64
go build -o "%OUTPUT_DIR%\blackjack-windows-amd64.exe" .
if %ERRORLEVEL% EQU 0 (
    echo ✅ Windows ^(amd64^) 编译成功: %OUTPUT_DIR%\blackjack-windows-amd64.exe
) else (
    echo ❌ Windows ^(amd64^) 编译失败
    exit /b 1
)

echo.
echo ✅ 编译完成!
echo 📂 输出目录: %OUTPUT_DIR%
echo.
dir "%OUTPUT_DIR%"
echo.
echo 🚀 使用方法:
echo    Linux:   cd %OUTPUT_DIR% ^&^& ./start.sh
echo    Windows: cd %OUTPUT_DIR% ^&^& blackjack-windows-amd64.exe
