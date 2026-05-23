@echo off
REM usage: build_windows        production (stripped, no swagger)
REM        build_windows dev    development (with swagger)

if "%1"=="dev" (
    echo [DEV BUILD] with Swagger
    go build -o xiaoquan-backend.exe
) else (
    echo [PROD BUILD] stripped, no Swagger
    go build -ldflags="-s -w" -tags noswagger -o xiaoquan-backend.exe
)

if %ERRORLEVEL% EQU 0 (
    echo [OK] xiaoquan-backend.exe
) else (
    echo [FAILED]
    exit /b 1
)
