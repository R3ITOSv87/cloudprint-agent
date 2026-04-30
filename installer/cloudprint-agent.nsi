; CloudPrint Agent Windows Installer
; Build: makensis -DVERSION="1.0.0" cloudprint-agent.nsi

!ifndef VERSION
  !define VERSION "1.0.0"
!endif

Name "CloudPrint Agent"
OutFile "CloudPrintAgent-Setup.exe"
InstallDir "$PROGRAMFILES64\CloudPrint"
InstallDirRegKey HKLM "Software\CloudPrint\Agent" ""
RequestExecutionLevel admin
SetCompressor /SOLID lzma

!include "MUI2.nsh"

; UI Pages
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "..\LICENSE"
!insertmacro MUI_PAGE_DIRECTORY

; Custom token page
Page custom TokenPage TokenPageLeave

!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "German"
!insertmacro MUI_LANGUAGE "English"

; Token page variables
Var Dialog
Var TokenLabel
Var TokenText
Var NameLabel
Var NameText
Var ApiUrlLabel
Var ApiUrlText
Var InstallToken
Var AgentName
Var ApiUrl

Function TokenPage
  nsDialogs::Create 1018
  Pop $Dialog

  ${If} $Dialog == error
    Abort
  ${EndIf}

  ${NSD_CreateLabel} 0 0 100% 12u "Install-Token (aus dem CloudPrint Dashboard):"
  Pop $TokenLabel

  ${NSD_CreateText} 0 13u 100% 12u ""
  Pop $TokenText

  ${NSD_CreateLabel} 0 32u 100% 12u "Agent-Name (z.B. Büro Erdgeschoss):"
  Pop $NameLabel

  ${NSD_CreateText} 0 45u 100% 12u "Mein Standort"
  Pop $NameText

  ${NSD_CreateLabel} 0 64u 100% 12u "API URL:"
  Pop $ApiUrlLabel

  ${NSD_CreateText} 0 77u 100% 12u "https://api.base44.com/api/apps/YOUR_APP_ID/functions"
  Pop $ApiUrlText

  nsDialogs::Show
FunctionEnd

Function TokenPageLeave
  ${NSD_GetText} $TokenText $InstallToken
  ${NSD_GetText} $NameText $AgentName
  ${NSD_GetText} $ApiUrlText $ApiUrl

  ${If} $InstallToken == ""
    MessageBox MB_ICONSTOP "Bitte gib einen Install-Token ein."
    Abort
  ${EndIf}
  ${If} $AgentName == ""
    MessageBox MB_ICONSTOP "Bitte gib einen Agent-Namen ein."
    Abort
  ${EndIf}
FunctionEnd

Section "CloudPrint Agent" SecMain
  SetOutPath "$INSTDIR"

  ; Copy binary
  File "cloudprint-agent.exe"

  ; Write uninstaller
  WriteUninstaller "$INSTDIR\Uninstall.exe"

  ; Registry entries
  WriteRegStr HKLM "Software\CloudPrint\Agent" "" "$INSTDIR"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CloudPrintAgent" \
    "DisplayName" "CloudPrint Agent"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CloudPrintAgent" \
    "UninstallString" "$INSTDIR\Uninstall.exe"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CloudPrintAgent" \
    "DisplayVersion" "${VERSION}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CloudPrintAgent" \
    "Publisher" "CloudPrint"

  ; Run registration
  DetailPrint "Registriere Agent bei CloudPrint..."
  nsExec::ExecToLog '"$INSTDIR\cloudprint-agent.exe" register --token "$InstallToken" --name "$AgentName" --api-url "$ApiUrl"'
  Pop $0
  ${If} $0 != 0
    MessageBox MB_ICONEXCLAMATION "Registrierung fehlgeschlagen (Code $0). Bitte prüfe Token und Netzwerkverbindung.$\nDu kannst die Registrierung manuell wiederholen:$\ncloudprint-agent.exe register --token TOKEN --name NAME --api-url URL"
  ${Else}
    DetailPrint "Registrierung erfolgreich!"
  ${EndIf}

  ; Install as Windows service
  DetailPrint "Installiere Windows-Dienst..."
  nsExec::ExecToLog '"$INSTDIR\cloudprint-agent.exe" install-service'
  nsExec::ExecToLog '"$INSTDIR\cloudprint-agent.exe" start-service'

  ; Start Menu shortcut
  CreateDirectory "$SMPROGRAMS\CloudPrint"
  CreateShortcut "$SMPROGRAMS\CloudPrint\CloudPrint Agent Status.lnk" \
    "$INSTDIR\cloudprint-agent.exe" "status" \
    "$INSTDIR\cloudprint-agent.exe" 0
  CreateShortcut "$SMPROGRAMS\CloudPrint\Drucker entdecken.lnk" \
    "$INSTDIR\cloudprint-agent.exe" "discover" \
    "$INSTDIR\cloudprint-agent.exe" 0
  CreateShortcut "$SMPROGRAMS\CloudPrint\Deinstallieren.lnk" \
    "$INSTDIR\Uninstall.exe" "" \
    "$INSTDIR\Uninstall.exe" 0
SectionEnd

Section "Uninstall"
  nsExec::ExecToLog '"$INSTDIR\cloudprint-agent.exe" stop-service'
  nsExec::ExecToLog '"$INSTDIR\cloudprint-agent.exe" uninstall-service'

  Delete "$INSTDIR\cloudprint-agent.exe"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir /r "$INSTDIR"

  Delete "$SMPROGRAMS\CloudPrint\CloudPrint Agent Status.lnk"
  Delete "$SMPROGRAMS\CloudPrint\Drucker entdecken.lnk"
  Delete "$SMPROGRAMS\CloudPrint\Deinstallieren.lnk"
  RMDir "$SMPROGRAMS\CloudPrint"

  DeleteRegKey HKLM "Software\CloudPrint\Agent"
  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CloudPrintAgent"
SectionEnd
