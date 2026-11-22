#ifndef AppVersion
  #define AppVersion "0.1.0"
#endif
[Setup]
AppId={{2B3BC294-C39D-4F67-8C47-FD2964894D2F}
AppName=DaisText
AppVersion={#AppVersion}
AppPublisher=DaisText contributors
AppPublisherURL=https://github.com/brenth82/daistext
DefaultDirName={localappdata}\Programs\DaisText
DefaultGroupName=DaisText
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
OutputDir=..\dist
OutputBaseFilename=DaisText-Setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\DaisText.exe
CloseApplications=yes
RestartApplications=no
AppMutex=Local\DaisText.Running
DisableProgramGroupPage=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
[Types]
Name: "standard"; Description: "DaisText and PDF export"
Name: "braille"; Description: "DaisText, PDF and BRF braille export"
Name: "custom"; Description: "Custom installation"; Flags: iscustom
[Components]
Name: "main"; Description: "DaisText and PDF export"; Types: standard braille custom; Flags: fixed
Name: "liblouis"; Description: "Liblouis braille translator and tables (offline BRF export)"; Types: braille
[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; Flags: unchecked
[Files]
Source: "..\dist\typst-files.txt"; Flags: dontcopy
Source: "..\dist\liblouis-files.txt"; Flags: dontcopy
Source: "..\dist\DaisText.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\daistext-cli.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\THIRD-PARTY-NOTICES.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\tools\*"; DestDir: "{app}\tools"; Check: NeedsTypst; Flags: ignoreversion recursesubdirs createallsubdirs solidbreak
Source: "..\dist\liblouis\*"; DestDir: "{app}\tools\liblouis"; Components: liblouis; Check: NeedsLiblouis; Flags: ignoreversion recursesubdirs createallsubdirs solidbreak
[Icons]
Name: "{group}\DaisText"; Filename: "{app}\DaisText.exe"
Name: "{group}\Uninstall DaisText"; Filename: "{uninstallexe}"
Name: "{autodesktop}\DaisText"; Filename: "{app}\DaisText.exe"; Tasks: desktopicon
[Run]
Filename: "{app}\DaisText.exe"; Description: "Open DaisText"; Flags: nowait postinstall skipifsilent
Filename: "{app}\DaisText.exe"; Flags: nowait; Check: IsUpdate

[UninstallDelete]
; Delete only DaisText-owned settings/cache locations, never output or input paths.
Type: files; Name: "{userappdata}\DaisText\settings.json"
Type: files; Name: "{userappdata}\DaisText\settings-*"
Type: dirifempty; Name: "{userappdata}\DaisText"
Type: filesandordirs; Name: "{localappdata}\DaisText\updates"
Type: filesandordirs; Name: "{localappdata}\DaisText\temp"
Type: dirifempty; Name: "{localappdata}\DaisText"

[Code]
var
  CheckedTypstDir, CheckedLiblouisDir: String;
  InstallTypst, InstallLiblouis: Boolean;

function ToolFilesMatch(const Dir, Manifest: String): Boolean;
var
  Entries: TArrayOfString;
  ManifestPath, Filename, Entry: String;
  I: Integer;
begin
  Result := False;
  if not DirExists(Dir) then
    Exit;
  ManifestPath := ExpandConstant('{tmp}\') + Manifest;
  if not FileExists(ManifestPath) then
    ExtractTemporaryFile(Manifest);
  if not LoadStringsFromFile(ManifestPath, Entries) or (GetArrayLength(Entries) = 0) then
    Exit;
  for I := 0 to GetArrayLength(Entries) - 1 do begin
    Entry := Entries[I];
    Filename := AddBackslash(Dir) + Copy(Entry, 66, Length(Entry));
    if not FileExists(Filename) then
      Exit;
    try
      if CompareText(GetSHA256OfFile(Filename), Copy(Entry, 1, 64)) <> 0 then
        Exit;
    except
      Log('Could not verify ' + Filename + ': ' + GetExceptionMessage);
      Exit;
    end;
  end;
  Result := True;
  Log('Reusing matching bundled files in ' + Dir);
end;

function NeedsTypst: Boolean;
var
  Dir: String;
begin
  Dir := ExpandConstant('{app}\tools');
  if CheckedTypstDir <> Dir then begin
    InstallTypst := not ToolFilesMatch(Dir, 'typst-files.txt');
    CheckedTypstDir := Dir;
  end;
  Result := InstallTypst;
end;

function NeedsLiblouis: Boolean;
var
  Dir: String;
begin
  Dir := ExpandConstant('{app}\tools\liblouis');
  if CheckedLiblouisDir <> Dir then begin
    InstallLiblouis := not ToolFilesMatch(Dir, 'liblouis-files.txt');
    CheckedLiblouisDir := Dir;
  end;
  Result := InstallLiblouis;
end;

function IsUpdate: Boolean;
begin
  Result := ExpandConstant('{param:DAISTEXTUPDATE|0}') = '1';
end;
