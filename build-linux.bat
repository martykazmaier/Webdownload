@echo off
setlocal
cd /d "%~dp0"
if exist "%~dp0.go-sdk\go\bin\go.exe" (
  set "PATH=%~dp0.go-sdk\go\bin;%PATH%"
)
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64
go build -trimpath -ldflags="-s -w" -o webdownload-linux-amd64 .
if errorlevel 1 exit /b 1
set GOARCH=386
go build -trimpath -ldflags="-s -w" -o webdownload-linux-386 .
if errorlevel 1 exit /b 1
set GOARCH=arm64
go build -trimpath -ldflags="-s -w" -o webdownload-linux-arm64 .
if errorlevel 1 exit /b 1
echo Built Linux webdownload-linux-amd64, webdownload-linux-386, and webdownload-linux-arm64
