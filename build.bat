@echo off
setlocal
cd /d "%~dp0"
if exist "%~dp0.go-sdk\go\bin\go.exe" (
  set "PATH=%~dp0.go-sdk\go\bin;%PATH%"
)
set GOOS=windows
set GOARCH=386
set CGO_ENABLED=0
go build -trimpath -ldflags="-s -w" -o webdownload.exe .
if errorlevel 1 exit /b 1
echo Built 32-bit webdownload.exe
