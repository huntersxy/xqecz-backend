@echo off
set GOOS=linux
set GOARCH=amd64
go build -o xiaoquan-backend main.go
if %errorlevel% equ 0 (
copy xiaoquan-backend ..\xiaoquan-backend-deploy\
echo Build successful!
) else (
echo Build failed!
)
pause