@echo off
setlocal EnableExtensions

REM ---------------------------------------------------------------
REM Build Drainy.exe with the embedded app.ico (no console window),
REM then build the installer dist\Drainy-Setup.exe if Inno Setup 6
REM is installed. Run from anywhere; the script switches to its folder.
REM ---------------------------------------------------------------

cd /d "%~dp0"

REM Toolchain: w64devkit provides gcc (cgo, required by Fyne) and windres.
set "W64DEVKIT=C:\w64devkit\bin"
if exist "%W64DEVKIT%\gcc.exe" set "PATH=%W64DEVKIT%;%PATH%"

set "CGO_ENABLED=1"
set "GOTOOLCHAIN=local"

if not exist "app.ico" (
    echo [ERROR] app.ico not found in %CD%
    exit /b 1
)
if not exist "main.rc" (
    echo [ERROR] main.rc not found in %CD%
    exit /b 1
)

where go >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Go is not on PATH.
    exit /b 1
)

REM 1. Make sure the rsrc tool is available (fallback helper, see step 2).
where rsrc >nul 2>&1
if errorlevel 1 (
    echo [INFO] rsrc not found, installing github.com/akavel/rsrc@latest ...
    go install github.com/akavel/rsrc@latest
    if errorlevel 1 (
        echo [ERROR] Failed to install rsrc.
        exit /b 1
    )
)

REM rsrc is installed into GOPATH\bin; make sure that folder is on PATH.
for /f "delims=" %%i in ('go env GOPATH') do set "GOPATH_DIR=%%i"
set "PATH=%GOPATH_DIR%\bin;%PATH%"

REM 2. Compile the resources into rsrc.syso.
REM    Prefer windres (reads main.rc, the source of truth). Fall back to rsrc
REM    (reads app.ico directly) if windres is missing.
if exist "rsrc.syso" del /q "rsrc.syso"

where windres >nul 2>&1
if not errorlevel 1 (
    echo [INFO] Compiling main.rc with windres ...
    windres -i main.rc -O coff -o rsrc.syso
) else (
    echo [INFO] windres not found, compiling app.ico with rsrc ...
    rsrc -ico app.ico -o rsrc.syso
)
if errorlevel 1 (
    echo [ERROR] Resource compilation failed.
    exit /b 1
)
if not exist "rsrc.syso" (
    echo [ERROR] rsrc.syso was not created.
    exit /b 1
)

REM 3. Final build: no console window (-H=windowsgui), icon from rsrc.syso.
echo [INFO] Building Drainy.exe ...
go build -ldflags="-H=windowsgui" -o Drainy.exe .
if errorlevel 1 (
    echo [ERROR] Build failed.
    exit /b 1
)
echo [OK] Drainy.exe built: %CD%\Drainy.exe

REM 4. Installer (optional): needs Inno Setup 6.
set "ISCC="
if exist "%ProgramFiles(x86)%\Inno Setup 6\ISCC.exe" set "ISCC=%ProgramFiles(x86)%\Inno Setup 6\ISCC.exe"
if not defined ISCC if exist "%ProgramFiles%\Inno Setup 6\ISCC.exe" set "ISCC=%ProgramFiles%\Inno Setup 6\ISCC.exe"
if not defined ISCC goto :noinstaller

REM Version comes from FyneApp.toml (single source of truth).
set "APP_VERSION="
for /f "tokens=1,* delims==" %%A in ('findstr /B /C:"Version" FyneApp.toml') do set "APP_VERSION=%%~B"
set "APP_VERSION=%APP_VERSION: =%"
set "APP_VERSION=%APP_VERSION:"=%"
if not defined APP_VERSION (
    echo [ERROR] Version not found in FyneApp.toml
    exit /b 1
)

echo [INFO] Building installer v%APP_VERSION% with Inno Setup ...
"%ISCC%" /DMyAppVersion=%APP_VERSION% installer.iss
if errorlevel 1 (
    echo [ERROR] Installer build failed.
    exit /b 1
)
echo [OK] Installer built: %CD%\dist\Drainy-Setup.exe
goto :done

:noinstaller
echo [WARN] Inno Setup 6 not found, installer skipped.
echo        Install it from https://jrsoftware.org/isinfo.php and run build.bat again.

:done
endlocal
exit /b 0
