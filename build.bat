@echo off
setlocal enabledelayedexpansion
echo Building THRM...

REM Extract version from wails.json
for /f "tokens=2 delims=:, " %%a in ('findstr /C:"\"productVersion\"" wails.json') do (
    set VERSION=%%a
    set VERSION=!VERSION:"=!
)

if "!VERSION!"=="" (
    echo WARNING: Could not extract version from wails.json, using dev
    set VERSION=dev
) else (
    echo Building version: !VERSION!
)

set "BUILD_BIN=build\bin"
set LDFLAGS=-s -w -X github.com/TIANLI0/BS2PRO-Controller/internal/version.BuildVersion=!VERSION! -H=windowsgui

if not exist "!BUILD_BIN!" mkdir "!BUILD_BIN!"

REM Build core service first
echo Building core service...
go-winres make --in cmd/core/winres/winres.json --out cmd/core/rsrc
go build -trimpath -ldflags "!LDFLAGS!" -o "build/bin/THRM Core.exe" ./cmd/core/

REM Installer icon is still file-based; system notification icon is now embedded in THRM Core.exe
if not exist "build\windows\icon.ico" (
    echo WARNING: build\windows\icon.ico not found, executable/installer icon may be incorrect
)

REM Build main application with wails
echo Building main application...
wails build -nsis -ldflags "!LDFLAGS!"

REM Ensure core service is in the bin directory for installer
if exist "build\bin\THRM Core.exe" (
    echo Core service built successfully
) else (
    echo ERROR: Core service build failed!
    exit /b 1
)

REM Keep build/bin focused on the current build. Old versioned executables are not used by NSIS
REM and make the build output look much larger than the actual distributable payload.
echo Cleaning stale release artifacts...
for %%F in (
    "!BUILD_BIN!\THRM-v*.exe"
    "!BUILD_BIN!\BS2PRO-Controller-v*.exe"
    "!BUILD_BIN!\BS2PRO-Controller-*-installer.exe"
    "!BUILD_BIN!\BS2PRO-Controller-amd64-installer.zip"
    "!BUILD_BIN!\BS2PRO-Controller.exe"
    "!BUILD_BIN!\BS2PRO-Core.exe"
    "!BUILD_BIN!\BS2PRO-Watchdog.exe"
    "!BUILD_BIN!\*.exe~"
) do (
    if exist "%%~F" del /q "%%~F"
)

echo Build completed successfully with version !VERSION!
endlocal