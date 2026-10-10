; Inno Setup script for Drainy.
; Build: ISCC.exe installer.iss  (build.bat runs it automatically if Inno Setup 6 is installed)
; Result: dist\Drainy-Setup.exe

#define MyAppName "Drainy"
#define MyAppExe "Drainy.exe"
; MyAppVersion is passed by build.bat from FyneApp.toml (single source of truth).
#ifndef MyAppVersion
  #define MyAppVersion "0.0.0"
#endif

[Setup]
; Fixed GUID: keeps upgrades and uninstall working across versions. Do not change it.
AppId={{B7E4C2A1-5D3F-4A8E-9C61-2F0D7A9E4B13}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppName}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
; Per-user install: no UAC prompt, installs under %LOCALAPPDATA%\Programs.
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
SetupIconFile=app.ico
UninstallDisplayIcon={app}\{#MyAppExe}
UninstallDisplayName={#MyAppName}
OutputDir=dist
OutputBaseFilename=Drainy-Setup
ShowLanguageDialog=auto

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "russian"; MessagesFile: "compiler:Languages\Russian.isl"

[Files]
Source: "Drainy.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExe}"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"

[Run]
Filename: "{app}\{#MyAppExe}"; Description: "{cm:LaunchProgram,{#MyAppName}}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
; Close a running Drainy first, otherwise Drainy.exe is locked and survives uninstall.
Filename: "{sys}\taskkill.exe"; Parameters: "/F /IM {#MyAppExe}"; Flags: runhidden; RunOnceId: "KillDrainy"

[UninstallDelete]
; Remove anything the app created next to the exe during its run.
Type: filesandordirs; Name: "{app}"
