@echo off
REM usage: build-linux           production (stripped, no swagger)
REM        build-linux dev       development (with swagger)

set GOOS=linux
set GOARCH=amd64

if "%1"=="dev" (
    echo [DEV BUILD] with Swagger
    go build -o xiaoquan-backend .
) else (
    echo [PROD BUILD] stripped, no Swagger
    go build -ldflags="-s -w" -tags noswagger -o xiaoquan-backend .
)

if %errorlevel% equ 0 (
    copy xiaoquan-backend ..\xiaoquan-backend-deploy\
    echo [OK] xiaoquan-backend copied to deploy dir
) else (
    echo [FAILED]
)
pause
