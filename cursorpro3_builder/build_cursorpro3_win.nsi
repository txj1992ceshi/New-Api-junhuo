Unicode True
ManifestDPIAware True
RequestExecutionLevel admin

!include "MUI2.nsh"

!define APP_NAME "CursorPro3 x64"
!define APP_EXE "CursorPro.exe"
!define COMPANY_NAME "yuxin"
!define INSTALL_DIR "$PROGRAMFILES64\\CursorPro3 x64"
!define OUT_FILE "/Users/jj/Documents/CursorPro3/CursorPro3 x64-setup.exe"
!define STAGE_DIR "/Users/jj/Documents/Playground/cursorpro3_builder/win_stage"
!define APP_ICON "${STAGE_DIR}\\CursorPro3.ico"

Name "${APP_NAME}"
OutFile "${OUT_FILE}"
InstallDir "${INSTALL_DIR}"
InstallDirRegKey HKLM "Software\\${COMPANY_NAME}\\${APP_NAME}" "Install_Dir"
BrandingText "CursorPro3 x64 Installer"
Icon "${APP_ICON}"
UninstallIcon "${APP_ICON}"
!define MUI_ICON "${APP_ICON}"
!define MUI_UNICON "${APP_ICON}"

!define MUI_ABORTWARNING
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"

Section "Install"
  SetOutPath "$INSTDIR"
  File "${STAGE_DIR}\\CursorPro.exe"
  File "${STAGE_DIR}\\CursorPro3.ico"
  File "${STAGE_DIR}\\tutorial_local.html"
  File "${STAGE_DIR}\\cursorpro3_control.ps1"
  File "${STAGE_DIR}\\cursorpro3_register_worker.ps1"
  File "${STAGE_DIR}\\start_cursorpro3_control.cmd"
  File "${STAGE_DIR}\\stop_cursorpro3_control.cmd"
  File "${STAGE_DIR}\\cursorpro3_endpoints.txt"
  File "${STAGE_DIR}\\cursorpro3_layout.txt"

  WriteRegStr HKLM "Software\\${COMPANY_NAME}\\${APP_NAME}" "Install_Dir" "$INSTDIR"
  WriteUninstaller "$INSTDIR\\Uninstall CursorPro3 x64.exe"
  CreateDirectory "$SMPROGRAMS\\${APP_NAME}"
  CreateShortcut "$SMPROGRAMS\\${APP_NAME}\\${APP_NAME}.lnk" "$INSTDIR\\${APP_EXE}" "" "$INSTDIR\\CursorPro3.ico"
  CreateShortcut "$SMPROGRAMS\\${APP_NAME}\\Start CursorPro3 Control.lnk" "$INSTDIR\\start_cursorpro3_control.cmd" "" "$INSTDIR\\CursorPro3.ico"
  CreateShortcut "$SMPROGRAMS\\${APP_NAME}\\Stop CursorPro3 Control.lnk" "$INSTDIR\\stop_cursorpro3_control.cmd" "" "$INSTDIR\\CursorPro3.ico"
  CreateShortcut "$DESKTOP\\${APP_NAME}.lnk" "$INSTDIR\\${APP_EXE}" "" "$INSTDIR\\CursorPro3.ico"
SectionEnd

Section "Uninstall"
  Delete "$DESKTOP\\${APP_NAME}.lnk"
  Delete "$SMPROGRAMS\\${APP_NAME}\\${APP_NAME}.lnk"
  Delete "$SMPROGRAMS\\${APP_NAME}\\Start CursorPro3 Control.lnk"
  Delete "$SMPROGRAMS\\${APP_NAME}\\Stop CursorPro3 Control.lnk"
  RMDir "$SMPROGRAMS\\${APP_NAME}"

  Delete "$INSTDIR\\${APP_EXE}"
  Delete "$INSTDIR\\CursorPro3.ico"
  Delete "$INSTDIR\\tutorial_local.html"
  Delete "$INSTDIR\\cursorpro3_control.ps1"
  Delete "$INSTDIR\\cursorpro3_register_worker.ps1"
  Delete "$INSTDIR\\start_cursorpro3_control.cmd"
  Delete "$INSTDIR\\stop_cursorpro3_control.cmd"
  Delete "$INSTDIR\\cursorpro3_endpoints.txt"
  Delete "$INSTDIR\\cursorpro3_layout.txt"
  Delete "$INSTDIR\\Uninstall CursorPro3 x64.exe"
  RMDir "$INSTDIR"

  DeleteRegKey HKLM "Software\\${COMPANY_NAME}\\${APP_NAME}"
SectionEnd
