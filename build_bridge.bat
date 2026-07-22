@echo off
setlocal
echo Building TempBridge with latest LibreHardwareMonitor...

set "ROOT=%~dp0"
set "PROJECT=%ROOT%bridge\TempBridge\TempBridge.csproj"
set "OUTDIR=%ROOT%build\bin\bridge"
set "BUILDROOT=%ROOT%build\bin"
set "TEMPROOT=%ROOT%temp"
rem THRM fork of LibreHardwareMonitor. Its default branch main = upstream master
rem + PR #2396 (Fire Range / Zen 5 X3D CPU power & topology). See temp\LibreHardwareMonitor.
set "LHM_URL=https://github.com/TIANLI0/LibreHardwareMonitor.git"
set "LHM_BRANCH=main"
set "LHM_REPO=%TEMPROOT%\LibreHardwareMonitor"
if defined LIBRE_HARDWARE_MONITOR_REPO set "LHM_REPO=%LIBRE_HARDWARE_MONITOR_REPO%"
set "LHM_PROJECT=%LHM_REPO%\LibreHardwareMonitorLib\LibreHardwareMonitorLib.csproj"
set "PAWNIO_URL=https://github.com/namazso/PawnIO.Setup/releases/latest/download/PawnIO_setup.exe"
set "PAWNIO_OUT=%BUILDROOT%\PawnIO_setup.exe"

if not exist "%BUILDROOT%" mkdir "%BUILDROOT%"
if not exist "%TEMPROOT%" mkdir "%TEMPROOT%"
if exist "%OUTDIR%" rmdir /s /q "%OUTDIR%"
if not exist "%OUTDIR%" mkdir "%OUTDIR%"

where git >nul 2>nul
if errorlevel 1 (
	echo ERROR: git not found. Cannot sync LibreHardwareMonitor HEAD.
	goto :error
)

if not exist "%LHM_REPO%\.git" (
	echo Cloning LibreHardwareMonitor into %LHM_REPO%...
	git clone --depth 1 --branch %LHM_BRANCH% "%LHM_URL%" "%LHM_REPO%"
	if errorlevel 1 goto :error
) else (
	rem Force the existing checkout onto %LHM_URL%@%LHM_BRANCH% even if it was
	rem previously pointed at upstream. -B + FETCH_HEAD discards any local divergence.
	echo Updating LibreHardwareMonitor in %LHM_REPO% from %LHM_URL% branch %LHM_BRANCH%...
	git -C "%LHM_REPO%" remote set-url origin "%LHM_URL%"
	if errorlevel 1 goto :error
	git -C "%LHM_REPO%" fetch --depth 1 origin %LHM_BRANCH%
	if errorlevel 1 goto :error
	git -C "%LHM_REPO%" checkout -B %LHM_BRANCH% FETCH_HEAD
	if errorlevel 1 goto :error
)

if not exist "%LHM_PROJECT%" (
	echo ERROR: LibreHardwareMonitorLib project not found at %LHM_PROJECT%
	goto :error
)

for /f %%i in ('git -C "%LHM_REPO%" rev-parse HEAD') do set "LHM_COMMIT=%%i"
echo Using LibreHardwareMonitor commit: %LHM_COMMIT%

echo Restoring TempBridge dependencies...
dotnet restore "%PROJECT%" /p:Platform=x64 /p:UseLibreHardwareMonitorProjectReference=true /p:LibreHardwareMonitorRepoRoot="%LHM_REPO%"
if errorlevel 1 goto :error

echo Publishing TempBridge...
dotnet publish "%PROJECT%" -c Release --self-contained false -o "%OUTDIR%" /p:Platform=x64 /p:DebugType=none /p:DebugSymbols=false /p:UseLibreHardwareMonitorProjectReference=true /p:LibreHardwareMonitorRepoRoot="%LHM_REPO%"
if errorlevel 1 goto :error

echo Removing non-runtime bridge artifacts...
del /q "%OUTDIR%\*.pdb" 2>nul
del /q "%OUTDIR%\*.xml" 2>nul
for /d %%D in ("%OUTDIR%\??-??") do if exist "%%~D" rmdir /s /q "%%~D"

echo Downloading PawnIO installer...
powershell -NoProfile -ExecutionPolicy Bypass -Command "try { [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -Uri '%PAWNIO_URL%' -OutFile '%PAWNIO_OUT%' -UseBasicParsing; exit 0 } catch { Write-Error $_; exit 1 }"
if errorlevel 1 goto :error

if not exist "%PAWNIO_OUT%" goto :error
echo PawnIO installer saved to: %PAWNIO_OUT%

echo Build completed. Output directory: %OUTDIR%
goto :end

:error
echo Build failed. See the output above.
exit /b 1

:end
endlocal
