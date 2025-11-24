; Edge Video NSIS Installer Script
; Builds a Windows installer that installs Edge Video as a Windows Service

!define PRODUCT_NAME "Edge Video"
!define PRODUCT_PUBLISHER "T3 Labs" 
!define PRODUCT_WEB_SITE "https://github.com/T3-Labs/edge-video"
!define PRODUCT_DIR_REGKEY "Software\Microsoft\Windows\CurrentVersion\App Paths\edge-video-service.exe"
!define PRODUCT_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}"
!define PRODUCT_UNINST_ROOT_KEY "HKLM"

; Get version from build environment or default
!ifndef PRODUCT_VERSION
!define PRODUCT_VERSION "1.2.0"
!endif

; Normalize version to 4-part format (X.Y.Z.W) required by VIProductVersion
; Simplificado: assume formato semântico X.Y.Z e adiciona .0
!define PRODUCT_VERSION_4PART "${PRODUCT_VERSION}.0"

SetCompressor lzma

; Modern UI
!include "MUI2.nsh"
!include "Sections.nsh"
!include "LogicLib.nsh"
!include "WinVer.nsh"

; Modern UI Configuration
!define MUI_ABORTWARNING
;!define MUI_ICON "${WORKSPACE_DIR}\installer\windows\edge-video.ico"
;!define MUI_UNICON "${WORKSPACE_DIR}\installer\windows\edge-video.ico"

; Welcome page
!insertmacro MUI_PAGE_WELCOME

; License page  
!insertmacro MUI_PAGE_LICENSE "..\..\LICENSE"

; Components page
!insertmacro MUI_PAGE_COMPONENTS

; Directory page
!insertmacro MUI_PAGE_DIRECTORY

; Install files page
!insertmacro MUI_PAGE_INSTFILES

; Finish page
!define MUI_FINISHPAGE_RUN "$INSTDIR\edge-video-service.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Install and start Edge Video Service"
!define MUI_FINISHPAGE_RUN_PARAMETERS "install"
!define MUI_FINISHPAGE_SHOWREADME "$INSTDIR\README.txt"
!insertmacro MUI_PAGE_FINISH

; Uninstaller pages
!insertmacro MUI_UNPAGE_INSTFILES

; Language
!insertmacro MUI_LANGUAGE "English"

; Installer details
Name "${PRODUCT_NAME} ${PRODUCT_VERSION}"
OutFile "..\..\dist\EdgeVideoSetup-${PRODUCT_VERSION}.exe"
InstallDir "$PROGRAMFILES64\T3Labs\EdgeVideo"
InstallDirRegKey HKLM "${PRODUCT_DIR_REGKEY}" ""
ShowInstDetails show
ShowUnInstDetails show

; Version info
VIProductVersion "${PRODUCT_VERSION_4PART}"
VIAddVersionKey "ProductName" "${PRODUCT_NAME}"
VIAddVersionKey "Comments" "RTSP Camera Capture Service"
VIAddVersionKey "CompanyName" "${PRODUCT_PUBLISHER}"
VIAddVersionKey "LegalCopyright" "© 2024 ${PRODUCT_PUBLISHER}"
VIAddVersionKey "FileDescription" "${PRODUCT_NAME} Installer"
VIAddVersionKey "FileVersion" "${PRODUCT_VERSION_4PART}"
VIAddVersionKey "ProductVersion" "${PRODUCT_VERSION}"

; Request admin privileges
RequestExecutionLevel admin

Function .onInit
  ; Check Windows version (Windows 7/Server 2008 R2 or later)
  ${IfNot} ${AtLeastWin7}
    MessageBox MB_OK "This application requires Windows 7 or later."
    Quit
  ${EndIf}

  ; Check if running as administrator
  UserInfo::GetAccountType
  pop $0
  ${If} $0 != "admin"
    MessageBox MB_OK "Administrator privileges are required to install this service."
    SetErrorLevel 740 ; ERROR_ELEVATION_REQUIRED
    Quit
  ${EndIf}
FunctionEnd

; Main installation section
Section "Edge Video Service (Required)" SEC01
  SectionIn RO  ; Read-only section
  
  ; Stop existing service if running
  DetailPrint "Checking for existing service..."
  nsExec::ExecToStack '"sc" query "EdgeVideoService"'
  Pop $0 ; Return value
  ${If} $0 == 0
    DetailPrint "Stopping existing Edge Video Service..."
    nsExec::ExecToLog '"$INSTDIR\edge-video-service.exe" stop'
    Sleep 3000  ; Wait for service to stop
    
    DetailPrint "Uninstalling existing service..."
    nsExec::ExecToLog '"$INSTDIR\edge-video-service.exe" uninstall'
    Sleep 2000
  ${EndIf}

  ; Install files
  SetOutPath "$INSTDIR"
  SetOverwrite ifnewer
  
  File "..\..\dist\edge-video-service.exe"
  File "..\..\dist\edge-video.exe"
  
  ; Create configuration directory and example config
  CreateDirectory "$INSTDIR\config"
  File /oname=config\config.toml "..\..\config.toml"
  
  ; Create logs directory
  CreateDirectory "$INSTDIR\logs"
  
  ; Install documentation if available
  IfFileExists "..\..\README.md" 0 +2
    File /oname=README.txt "..\..\README.md"
  
  IfFileExists "..\..\docs\windows\*.*" 0 +3
    CreateDirectory "$INSTDIR\docs"
    File /r "..\..\docs\windows\*.*"
    
  ; Create uninstaller
  WriteUninstaller "$INSTDIR\uninstall.exe"
  
  ; Registry entries
  WriteRegStr HKLM "${PRODUCT_DIR_REGKEY}" "" "$INSTDIR\edge-video-service.exe"
  WriteRegStr ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "DisplayName" "$(^Name)"
  WriteRegStr ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "UninstallString" "$INSTDIR\uninstall.exe"
  WriteRegStr ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "DisplayIcon" "$INSTDIR\edge-video-service.exe"
  WriteRegStr ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "DisplayVersion" "${PRODUCT_VERSION}"
  WriteRegStr ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "Publisher" "${PRODUCT_PUBLISHER}"
  WriteRegStr ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "URLInfoAbout" "${PRODUCT_WEB_SITE}"
  
  ; Estimate install size (in KB)
  GetSize "$INSTDIR" "/S=0K" $0 $1 $2
  IntFmt $0 "0x%08X" $0
  WriteRegDWORD ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}" "EstimatedSize" "$0"
SectionEnd

; Optional: Start Menu shortcuts
Section "Start Menu Shortcuts" SEC02
  CreateDirectory "$SMPROGRAMS\T3Labs\Edge Video"
  CreateShortCut "$SMPROGRAMS\T3Labs\Edge Video\Edge Video Service Manager.lnk" "$INSTDIR\edge-video-service.exe" "" "$INSTDIR\edge-video-service.exe" 0
  CreateShortCut "$SMPROGRAMS\T3Labs\Edge Video\Configuration.lnk" "notepad.exe" "$INSTDIR\config\config.toml"
  CreateShortCut "$SMPROGRAMS\T3Labs\Edge Video\Uninstall.lnk" "$INSTDIR\uninstall.exe"
  
  ; Desktop shortcut for service management
  CreateShortCut "$DESKTOP\Edge Video Service.lnk" "$INSTDIR\edge-video-service.exe" "" "$INSTDIR\edge-video-service.exe" 0
SectionEnd

; Optional: Install as Windows Service
Section "Install and Start Service" SEC03
  DetailPrint "Installing Edge Video as Windows Service..."
  nsExec::ExecToLog '"$INSTDIR\edge-video-service.exe" install'
  Pop $0
  ${If} $0 != 0
    DetailPrint "Service installation failed with code $0"
    MessageBox MB_OK|MB_ICONEXCLAMATION "Failed to install Windows Service. You may need to install manually using: edge-video-service.exe install"
  ${Else}
    DetailPrint "Service installed successfully"
    
    ; Try to start the service
    DetailPrint "Starting Edge Video Service..."
    nsExec::ExecToLog '"$INSTDIR\edge-video-service.exe" start'
    Pop $0
    ${If} $0 != 0
      DetailPrint "Failed to start service automatically. You can start it manually from Services.msc or using: net start EdgeVideoService"
    ${Else}
      DetailPrint "Service started successfully"
    ${EndIf}
  ${EndIf}
SectionEnd

; Section descriptions
!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SEC01} "Core Edge Video service files and configuration."
  !insertmacro MUI_DESCRIPTION_TEXT ${SEC02} "Create shortcuts in Start Menu and Desktop for easy access."
  !insertmacro MUI_DESCRIPTION_TEXT ${SEC03} "Install Edge Video as a Windows Service and start automatically."
!insertmacro MUI_FUNCTION_DESCRIPTION_END

; Uninstaller
Function un.onInit
  MessageBox MB_ICONQUESTION|MB_YESNO|MB_DEFBUTTON2 "Are you sure you want to completely remove $(^Name) and all of its components?" IDYES +2
  Abort
FunctionEnd

Section Uninstall
  ; Stop and uninstall service
  DetailPrint "Stopping Edge Video Service..."
  nsExec::ExecToLog '"$INSTDIR\edge-video-service.exe" stop'
  Sleep 3000
  
  DetailPrint "Uninstalling Edge Video Service..."
  nsExec::ExecToLog '"$INSTDIR\edge-video-service.exe" uninstall'
  Sleep 2000

  ; Remove files
  Delete "$INSTDIR\edge-video-service.exe"
  Delete "$INSTDIR\edge-video.exe"
  Delete "$INSTDIR\uninstall.exe"
  Delete "$INSTDIR\README.txt"
  
  ; Remove directories (only if empty)
  RMDir /r "$INSTDIR\docs"
  RMDir "$INSTDIR\logs"
  
  ; Ask about configuration
  MessageBox MB_YESNO|MB_ICONQUESTION "Do you want to remove configuration files? (This will delete your camera settings)" IDNO +3
    RMDir /r "$INSTDIR\config"
    Goto +2
    DetailPrint "Configuration files preserved in $INSTDIR\config"
  
  RMDir "$INSTDIR"
  RMDir "$PROGRAMFILES64\T3Labs"

  ; Remove shortcuts
  Delete "$SMPROGRAMS\T3Labs\Edge Video\*"
  RMDir "$SMPROGRAMS\T3Labs\Edge Video"
  RMDir "$SMPROGRAMS\T3Labs"
  Delete "$DESKTOP\Edge Video Service.lnk"

  ; Remove registry entries
  DeleteRegKey ${PRODUCT_UNINST_ROOT_KEY} "${PRODUCT_UNINST_KEY}"
  DeleteRegKey HKLM "${PRODUCT_DIR_REGKEY}"
  
  SetAutoClose true
SectionEnd