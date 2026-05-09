@echo off
REM 小泉动漫后端 - Windows编译脚本

echo =========================================
echo 小泉动漫后端 - Windows构建
echo =========================================

echo 编译中...
go build -o xiaoquan-backend.exe

if %ERRORLEVEL% EQU 0 (
    echo =========================================
    echo ✓ 编译成功！
    echo 可执行文件: xiaoquan-backend.exe
    echo =========================================
) else (
    echo ✗ 编译失败！
    exit /b 1
)
