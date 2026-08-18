#ifndef MyAppVersion
  #define MyAppVersion "0.15.8.2"
#endif
#ifndef RepoRoot
  #define RepoRoot "."
#endif
#define MyAppName "AutoParts Store Agent"
#define MyAppPublisher "AutoParts"
#define MyWorkerExeName "AutoPartsStoreEdge.exe"
#define MyManagerExeName "AutoPartsStoreEdgeManager.exe"
#define MyUpdaterExeName "AutoPartsStoreEdgeUpdater.exe"
#define MyServiceName "AutoPartsStoreEdgeManager"
#define MyLegacyServiceName "AutoPartsStoreEdge"

[Setup]
AppId={{B837C5AB-C7A5-4D5E-A0BF-3B8F09B2CA74}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={autopf}\AutoParts\StoreAgent
DisableProgramGroupPage=yes
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
OutputDir={#RepoRoot}\dist
OutputBaseFilename=AutoParts-Store-Agent-Setup-{#MyAppVersion}
UninstallDisplayIcon={app}\{#MyWorkerExeName}
SetupLogging=yes
CloseApplications=no
RestartApplications=no

[Files]
Source: "{#RepoRoot}\dist\{#MyWorkerExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#RepoRoot}\dist\{#MyManagerExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#RepoRoot}\dist\{#MyUpdaterExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#RepoRoot}\edge\windows\service-install.ps1"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#RepoRoot}\edge\windows\service-remove.ps1"; DestDir: "{app}"; Flags: ignoreversion

[Dirs]
Name: "{commonappdata}\AutoParts\StoreEdge\data"
Name: "{commonappdata}\AutoParts\StoreEdge\logs"

[Icons]
Name: "{autodesktop}\AutoParts Offline POS"; Filename: "{app}\{#MyWorkerExeName}"; Parameters: "open"; WorkingDir: "{app}"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Create an AutoParts Offline POS desktop shortcut"; GroupDescription: "Shortcuts:"; Flags: checkedonce

[Run]
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\service-install.ps1"" -ManagerExecutable ""{app}\{#MyManagerExeName}"""; Flags: runhidden waituntilterminated
Filename: "{app}\{#MyWorkerExeName}"; Parameters: "open"; Description: "Open AutoParts Offline POS"; Flags: postinstall nowait skipifsilent

[UninstallRun]
Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\service-remove.ps1"""; Flags: runhidden waituntilterminated; RunOnceId: "RemoveAutoPartsStoreEdgeManagerService"

[Code]
function PrepareToInstall(var NeedsRestart: Boolean): String;
var ResultCode: Integer;
begin
  Result := '';
  Exec(ExpandConstant('{sys}\sc.exe'), 'stop {#MyServiceName}', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Exec(ExpandConstant('{sys}\sc.exe'), 'stop {#MyLegacyServiceName}', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Sleep(1200);
end;
