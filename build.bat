@echo off
setlocal enabledelayedexpansion
echo Building THRM...

REM build/config.yml info.version is the single source of the version (also used for exe resources and the installer).
set "PRODUCT_VERSION="
for /f "tokens=2 delims=:# " %%a in ('findstr /R /C:"^  *version:" build\config.yml') do (
    if "!PRODUCT_VERSION!"=="" set "PRODUCT_VERSION=%%~a"
)
if "!PRODUCT_VERSION!"=="" (
    echo ERROR: Could not extract info.version from build\config.yml
    exit /b 1
)

if not "%THRM_BUILD_VERSION%"=="" (
    set "VERSION=%THRM_BUILD_VERSION%"
    echo Building version from THRM_BUILD_VERSION: !VERSION!
) else (
    set "VERSION=!PRODUCT_VERSION!"
    echo Building version: !VERSION!
)

set "BUILD_BIN=build\bin"
set LDFLAGS=-s -w -X github.com/TIANLI0/THRM/internal/version.BuildVersion=!VERSION! -H=windowsgui

if not exist "!BUILD_BIN!" mkdir "!BUILD_BIN!"
if not exist "!BUILD_BIN!" (
    echo ERROR: Could not create !BUILD_BIN!
    exit /b 1
)

REM Build core service first
echo Building core service...
REM The core's version resource carries the same build/config.yml version as the GUI;
REM winres.json only holds the static fields (icon, manifest, names).
go-winres make --in cmd/core/winres/winres.json --out cmd/core/rsrc --product-version !PRODUCT_VERSION! --file-version !PRODUCT_VERSION!
if errorlevel 1 (
    echo ERROR: go-winres failed to generate the core service resources
    exit /b 1
)
go build -trimpath -ldflags "!LDFLAGS!" -o "build/bin/THRM Core.exe" ./cmd/core/
if errorlevel 1 (
    echo ERROR: Core service build failed
    exit /b 1
)

REM Installer icon is still file-based; system notification icon is now embedded in THRM Core.exe
if not exist "build\windows\icon.ico" (
    echo WARNING: build\windows\icon.ico not found, executable/installer icon may be incorrect
)

REM Build the GUI and the installer (FluentSetup) with the Wails v3 Taskfile
echo Building main application...
wails3 task package VERSION=!VERSION! PRODUCT_VERSION=!PRODUCT_VERSION!
if errorlevel 1 (
    echo ERROR: GUI build failed!
    exit /b 1
)

REM Ensure core service is in the bin directory for installer
if exist "build\bin\THRM Core.exe" (
    echo Core service built successfully
) else (
    echo ERROR: Core service build failed!
    exit /b 1
)

REM Keep build/bin focused on the current build. Old versioned executables are not used by the installer
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