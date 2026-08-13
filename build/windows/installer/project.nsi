Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them with the values from ProjectInfo.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
##
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the ProjectInfo file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "MyProject" # Default "{{.Name}}"
## !define INFO_COMPANYNAME    "MyCompany" # Default "{{.Info.CompanyName}}"
## !define INFO_PRODUCTNAME    "MyProduct" # Default "{{.Info.ProductName}}"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "{{.Info.ProductVersion}}"
## !define INFO_COPYRIGHT      "Copyright" # Default "{{.Info.Copyright}}"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## Override to prevent duplicate product names in registry key
!define UNINST_KEY_NAME "THRM"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
####
## Include the wails tools
####
!include "wails_tools.nsh"

# Include required plugins and libraries
!include "MUI.nsh"
!include "FileFunc.nsh"
!include "WordFunc.nsh"

# Include .NET Framework Detection
!include "DotNetChecker.nsh"
!include "project_strings.nsh"

!macro TryInstallDirCandidate CANDIDATE SOURCE LEGACY
    ${If} "${CANDIDATE}" != ""
        ${If} ${FileExists} "${CANDIDATE}\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR "${CANDIDATE}"
            ${If} "${LEGACY}" == "1"
                DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            ${Else}
                DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
            ${EndIf}
            ${If} "${LEGACY}" == "1"
                Goto found_legacy_installation
            ${Else}
                Goto found_installation
            ${EndIf}
        ${EndIf}
        ${If} ${FileExists} "${CANDIDATE}\THRM Core.exe"
            StrCpy $INSTDIR "${CANDIDATE}"
            ${If} "${LEGACY}" == "1"
                DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            ${Else}
                DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
            ${EndIf}
            ${If} "${LEGACY}" == "1"
                Goto found_legacy_installation
            ${Else}
                Goto found_installation
            ${EndIf}
        ${EndIf}
        ${If} ${FileExists} "${CANDIDATE}\BS2PRO-Controller.exe"
            StrCpy $INSTDIR "${CANDIDATE}"
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "${CANDIDATE}\BS2PRO-controller.exe"
            StrCpy $INSTDIR "${CANDIDATE}"
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "${CANDIDATE}\BS2PRO.exe"
            StrCpy $INSTDIR "${CANDIDATE}"
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "${CANDIDATE}\BS2PRO-Core.exe"
            StrCpy $INSTDIR "${CANDIDATE}"
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "${CANDIDATE}\uninstall.exe"
            StrCpy $INSTDIR "${CANDIDATE}"
            ${If} "${LEGACY}" == "1"
                DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            ${Else}
                DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
            ${EndIf}
            ${If} "${LEGACY}" == "1"
                Goto found_legacy_installation
            ${Else}
                Goto found_installation
            ${EndIf}
        ${EndIf}
    ${EndIf}
!macroend

# Built-in PawnIO version for install/update decisions.
# You can override this at build time with: -DPAWNIO_BUNDLED_VERSION=x.y.z
!ifndef PAWNIO_BUNDLED_VERSION
!define PAWNIO_BUNDLED_VERSION "2.2.0.0"
!endif

!ifndef CORE_EXECUTABLE_SOURCE
!define CORE_EXECUTABLE_SOURCE "..\..\bin\THRM Core.exe"
    !if /FileExists "${CORE_EXECUTABLE_SOURCE}"
    !else
        !undef CORE_EXECUTABLE_SOURCE
        !define CORE_EXECUTABLE_SOURCE "..\..\bin\BS2PRO-Core.exe"
    !endif
!endif

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXECUTABLE}"
!define MUI_FINISHPAGE_RUN_TEXT "$(THRM_STR_FINISHPAGE_RUN)"
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!define MUI_PAGE_CUSTOMFUNCTION_PRE WelcomePagePre
!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_COMPONENTS # Component selection page
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uinstalling page

!insertmacro MUI_LANGUAGE "SimpChinese"
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "Japanese"

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
Caption "$(THRM_STR_CAPTION)"
BrandingText "${INFO_PRODUCTNAME} v${INFO_PRODUCTVERSION}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
InstallDir "$PROGRAMFILES64\${INFO_PRODUCTNAME}" # Default installing folder (single level)
ShowInstDetails show # This will always show the installation details.

Var LegacyRenameNoticeNeeded

Function .onInit
    StrCpy $LegacyRenameNoticeNeeded "0"
   !insertmacro wails.checkArchitecture
   
   # Check for .NET Framework 4.7.2 or later
   !insertmacro CheckNetFramework 472
   Pop $0
   ${If} $0 == "false"
       MessageBox MB_OK|MB_ICONSTOP "$(THRM_STR_REQUIRE_DOTNET)"
       Abort
   ${EndIf}
   
    # Check for existing installation and set install directory
   Call DetectExistingInstallation
FunctionEnd

Function WelcomePagePre
    ${If} $LegacyRenameNoticeNeeded == "1"
        !insertmacro INSTALLOPTIONS_WRITE "ioSpecial.ini" "Field 2" "Text" "$(THRM_STR_LEGACY_TITLE)"
        !insertmacro INSTALLOPTIONS_WRITE "ioSpecial.ini" "Field 3" "Text" "$(THRM_STR_LEGACY_BODY)"
    ${EndIf}
FunctionEnd

# Function to clean up legacy/duplicate registry keys
Function CleanLegacyRegistryKeys
    DetailPrint "$(THRM_STR_CLEANING_LEGACY_REG)"
    SetRegView 64
    
    # List of known legacy/duplicate registry key names
    # BS2PRO-controllerBS2PRO-controller (duplicate product name)
    # TIANLI0BS2PRO-Controller (old company+product format)
    # BS2PRO-ControllerBS2PRO-Controller (case variation)
    
    Push $R0
    Push $R1
    
    # Check and remove BS2PRO-controllerBS2PRO-controller
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-controllerBS2PRO-controller" "UninstallString"
    ${If} $R0 != ""
        DetailPrint "$(THRM_STR_FOUND_REGKEY) BS2PRO-controllerBS2PRO-controller"
        DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-controllerBS2PRO-controller"
        DetailPrint "$(THRM_STR_REMOVED_REGKEY)"
    ${EndIf}

    # Check and remove BS2PRO-ControllerBS2PRO-Controller
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-ControllerBS2PRO-Controller" "UninstallString"
    ${If} $R0 != ""
        DetailPrint "$(THRM_STR_FOUND_REGKEY) BS2PRO-ControllerBS2PRO-Controller"
        DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-ControllerBS2PRO-Controller"
        DetailPrint "$(THRM_STR_REMOVED_REGKEY)"
    ${EndIf}

    # Check and remove BS2PRO-Controller (actual legacy product key)
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-Controller" "UninstallString"
    ${If} $R0 != ""
        DetailPrint "$(THRM_STR_FOUND_REGKEY) BS2PRO-Controller"
        DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-Controller"
        DetailPrint "$(THRM_STR_REMOVED_REGKEY)"
    ${EndIf}
    
    # Check and remove TIANLI0BS2PRO-Controller
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO-Controller" "UninstallString"
    ${If} $R0 != ""
        DetailPrint "$(THRM_STR_FOUND_REGKEY) TIANLI0BS2PRO-Controller"
        DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO-Controller"
        DetailPrint "$(THRM_STR_REMOVED_REGKEY)"
    ${EndIf}
    
    # Check and remove TIANLI0BS2PRO (current wails.json would generate this)
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO" "UninstallString"
    ${If} $R0 != ""
        DetailPrint "$(THRM_STR_FOUND_REGKEY) TIANLI0BS2PRO"
        DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO"
        DetailPrint "$(THRM_STR_REMOVED_REGKEY)"
    ${EndIf}
    
    Pop $R1
    Pop $R0
FunctionEnd

# Function to detect existing installation and set install directory
Function DetectExistingInstallation
    DetailPrint "$(THRM_STR_CHECKING_INSTALL)"
    SetRegView 64
    
    Push $R0
    Push $R1
    Push $R2

    # Show locally installed version if available
    ReadRegStr $R2 HKLM "${UNINST_KEY}" "DisplayVersion"
    ${If} $R2 == ""
        ReadRegStr $R2 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-controllerBS2PRO-controller" "DisplayVersion"
    ${EndIf}
    ${If} $R2 == ""
        ReadRegStr $R2 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-ControllerBS2PRO-Controller" "DisplayVersion"
    ${EndIf}
    ${If} $R2 == ""
        ReadRegStr $R2 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-Controller" "DisplayVersion"
    ${EndIf}
    ${If} $R2 == ""
        ReadRegStr $R2 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO-Controller" "DisplayVersion"
    ${EndIf}
    ${If} $R2 == ""
        ReadRegStr $R2 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO" "DisplayVersion"
    ${EndIf}
    ${If} $R2 != ""
        DetailPrint "$(THRM_STR_LOCAL_VERSION) $R2"
    ${Else}
        DetailPrint "$(THRM_STR_NO_LOCAL_VERSION)"
    ${EndIf}
    
    # First, check all possible registry keys to find installation path
    # DO NOT delete registry keys yet - we need them to find the install path!
    
    # Method 1: Try current registry key (THRM)
    ReadRegStr $R0 HKLM "${UNINST_KEY}" "InstallLocation"
    !insertmacro TryInstallDirCandidate "$R0" "正确键-安装位置" "0"
    ${If} $R0 != ""
        ${If} ${FileExists} "$R0\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R0
            DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R0\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R0
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
    ${EndIf}

    ReadRegStr $R0 HKLM "${UNINST_KEY}" "UninstallString"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        !insertmacro TryInstallDirCandidate "$R1" "正确键-卸载路径" "0"
        ${If} ${FileExists} "$R1\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R1
            DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
    ${EndIf}
    
    # Method 2: Check direct legacy registry key (BS2PRO-Controller)
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-Controller" "InstallLocation"
    !insertmacro TryInstallDirCandidate "$R0" "旧版键-安装位置" "1"

    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-Controller" "UninstallString"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        !insertmacro TryInstallDirCandidate "$R1" "旧版键-卸载路径" "1"
    ${EndIf}

    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-Controller" "DisplayIcon"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        !insertmacro TryInstallDirCandidate "$R1" "旧版键-图标路径" "1"
    ${EndIf}

    # Method 3: Check legacy/duplicate registry keys to find old installation
    # BS2PRO-controllerBS2PRO-controller (the current problematic key)
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-controllerBS2PRO-controller" "InstallLocation"
    !insertmacro TryInstallDirCandidate "$R0" "重复键-安装位置" "1"
    ${If} $R0 != ""
        ${If} ${FileExists} "$R0\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R0
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "$R0\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R0
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
    ${EndIf}

    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-controllerBS2PRO-controller" "UninstallString"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        !insertmacro TryInstallDirCandidate "$R1" "重复键-卸载路径" "1"
        ${If} ${FileExists} "$R1\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R1
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
    ${EndIf}
    
    # Try DisplayIcon from duplicate key
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-controllerBS2PRO-controller" "DisplayIcon"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        !insertmacro TryInstallDirCandidate "$R1" "重复键-图标路径" "1"
        ${If} ${FileExists} "$R1\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R1
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
    ${EndIf}

    # Method 3b: Check BS2PRO-ControllerBS2PRO-Controller (case-variant duplicate key)
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-ControllerBS2PRO-Controller" "InstallLocation"
    !insertmacro TryInstallDirCandidate "$R0" "大小写重复键-安装位置" "1"

    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-ControllerBS2PRO-Controller" "UninstallString"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        !insertmacro TryInstallDirCandidate "$R1" "大小写重复键-卸载路径" "1"
    ${EndIf}

    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-ControllerBS2PRO-Controller" "DisplayIcon"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        !insertmacro TryInstallDirCandidate "$R1" "大小写重复键-图标路径" "1"
    ${EndIf}
    
    # Method 4: Check TIANLI0BS2PRO-Controller (old company+product format)
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO-Controller" "InstallLocation"
    !insertmacro TryInstallDirCandidate "$R0" "旧格式键-安装位置" "1"
    ${If} $R0 != ""
        ${If} ${FileExists} "$R0\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R0
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "$R0\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R0
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
    ${EndIf}

    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO-Controller" "UninstallString"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        !insertmacro TryInstallDirCandidate "$R1" "旧格式键-卸载路径" "1"
        ${If} ${FileExists} "$R1\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R1
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
    ${EndIf}
    
    # Method 5: Check TIANLI0BS2PRO (wails.json generates this)
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO" "InstallLocation"
    !insertmacro TryInstallDirCandidate "$R0" "TIANLI0BS2PRO-安装位置" "1"
    ${If} $R0 != ""
        ${If} ${FileExists} "$R0\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R0
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "$R0\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R0
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
    ${EndIf}

    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO" "UninstallString"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        !insertmacro TryInstallDirCandidate "$R1" "TIANLI0BS2PRO-卸载路径" "1"
        ${If} ${FileExists} "$R1\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R1
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
    ${EndIf}
    
    # Second, try to read from DisplayIcon in uninstall registry
    ReadRegStr $R0 HKLM "${UNINST_KEY}" "DisplayIcon"
    ${If} $R0 != ""
        # Remove surrounding quotes
        Push $R0
        Call TrimQuotes
        Pop $R0
        
        ${GetParent} $R0 $R1  # Get parent directory
        !insertmacro TryInstallDirCandidate "$R1" "正确键-图标路径" "0"
        ${If} ${FileExists} "$R1\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R1
            DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
    ${EndIf}
    
    # Third, try to read InstallLocation from registry
    ReadRegStr $R0 HKLM "${UNINST_KEY}" "InstallLocation"
    !insertmacro TryInstallDirCandidate "$R0" "安装位置" "0"
    ${If} $R0 != ""
        ${If} ${FileExists} "$R0\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R0
            DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R0\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R0
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
    ${EndIf}
    
    # Fourth, check common installation locations (single level path)
    ${If} ${FileExists} "$PROGRAMFILES64\${INFO_PRODUCTNAME}\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES64\${INFO_PRODUCTNAME}"
        DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
        Goto found_installation
    ${EndIf}
    
    ${If} ${FileExists} "$PROGRAMFILES32\${INFO_PRODUCTNAME}\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES32\${INFO_PRODUCTNAME}"
        DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
        Goto found_installation
    ${EndIf}
    
    # Fifth, check legacy paths with Company\Product structure
    ${If} ${FileExists} "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
        DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
        Goto found_installation
    ${EndIf}

    !insertmacro TryInstallDirCandidate "$PROGRAMFILES64\TIANLI0\BS2PRO-Controller" "旧公司目录" "1"
    !insertmacro TryInstallDirCandidate "$PROGRAMFILES32\TIANLI0\BS2PRO-Controller" "旧公司目录" "1"
    !insertmacro TryInstallDirCandidate "$PROGRAMFILES64\BS2PRO-controller" "旧目录" "1"
    !insertmacro TryInstallDirCandidate "$PROGRAMFILES32\BS2PRO-controller" "旧目录" "1"
    
    # Sixth, try alternative common paths
    ${If} ${FileExists} "$PROGRAMFILES64\THRM\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES64\THRM"
        DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
        Goto found_installation
    ${EndIf}
    
    ${If} ${FileExists} "$PROGRAMFILES32\THRM\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES32\THRM"
        DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
        Goto found_installation
    ${EndIf}
    
    # Seventh, check for THRM Core.exe in common paths
    ${If} ${FileExists} "$PROGRAMFILES64\${INFO_PRODUCTNAME}\THRM Core.exe"
        StrCpy $INSTDIR "$PROGRAMFILES64\${INFO_PRODUCTNAME}"
        DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
        Goto found_installation
    ${EndIf}
    
    ${If} ${FileExists} "$PROGRAMFILES64\THRM\THRM Core.exe"
        StrCpy $INSTDIR "$PROGRAMFILES64\THRM"
        DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
        Goto found_installation
    ${EndIf}

    ${If} ${FileExists} "$PROGRAMFILES64\BS2PRO-Controller\BS2PRO-Controller.exe"
        StrCpy $INSTDIR "$PROGRAMFILES64\BS2PRO-Controller"
        DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
        Goto found_legacy_installation
    ${EndIf}

    ${If} ${FileExists} "$PROGRAMFILES32\BS2PRO-Controller\BS2PRO-Controller.exe"
        StrCpy $INSTDIR "$PROGRAMFILES32\BS2PRO-Controller"
        DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
        Goto found_legacy_installation
    ${EndIf}

    ${If} ${FileExists} "$PROGRAMFILES64\BS2PRO-Controller\BS2PRO-Core.exe"
        StrCpy $INSTDIR "$PROGRAMFILES64\BS2PRO-Controller"
        DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
        Goto found_legacy_installation
    ${EndIf}

    ${If} ${FileExists} "$PROGRAMFILES32\BS2PRO-Controller\BS2PRO-Core.exe"
        StrCpy $INSTDIR "$PROGRAMFILES32\BS2PRO-Controller"
        DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
        Goto found_legacy_installation
    ${EndIf}
    
    # If no existing installation found, use simple product name for directory
    # Use THRM as the default install directory
    StrCpy $INSTDIR "$PROGRAMFILES64\THRM"
    DetailPrint "$(THRM_STR_DEFAULT_DIR) $INSTDIR"
    Goto end_detection

    found_legacy_installation:
    StrCpy $LegacyRenameNoticeNeeded "1"
    Goto found_installation
    
    found_installation:
    DetailPrint "$(THRM_STR_UPGRADE_TARGET) $INSTDIR"
    # Now clean up legacy registry keys AFTER we've found the install path
    Call CleanLegacyRegistryKeys
    
    end_detection:
    Pop $R2
    Pop $R1
    Pop $R0
FunctionEnd

# Function to write current version info to uninstall registry key
Function WriteCurrentVersionInfo
    SetRegView 64
    WriteRegStr HKLM "${UNINST_KEY}" "DisplayVersion" "${INFO_PRODUCTVERSION}"
    WriteRegStr HKLM "${UNINST_KEY}" "Version" "${INFO_PRODUCTVERSION}"
    WriteRegStr HKLM "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
    WriteRegStr HKLM "${UNINST_KEY}" "DisplayName" "${INFO_PRODUCTNAME}"
    WriteRegStr HKLM "${UNINST_KEY}" "Publisher" "${INFO_COMPANYNAME}"
    DetailPrint "$(THRM_STR_WRITE_VERSION) ${INFO_PRODUCTVERSION}"
FunctionEnd

# Helper function to trim quotes from a string
Function TrimQuotes
    Exch $R0 ; Original string
    Push $R1
    Push $R2
    
    StrCpy $R1 $R0 1 ; First char
    StrCmp $R1 '"' 0 +2
    StrCpy $R0 $R0 "" 1 ; Remove first quote
    
    StrLen $R2 $R0
    IntOp $R2 $R2 - 1
    StrCpy $R1 $R0 1 $R2 ; Last char
    StrCmp $R1 '"' 0 +2
    StrCpy $R0 $R0 $R2 ; Remove last quote
    
    Pop $R2
    Pop $R1
    Exch $R0 ; Trimmed string
FunctionEnd

# Function to stop running application instances
Function StopRunningInstances
    DetailPrint "$(THRM_STR_CHECKING_PROCESSES)"
    
    # Try to stop the core service first (it manages the fan control)
    # Use /FI with proper error handling
    ClearErrors
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "THRM Core.exe" /T'
    Pop $0
    Pop $1
    ${If} $0 == 0
        DetailPrint "$(THRM_STR_CLOSE_CORE)"
        Sleep 2000
    ${EndIf}
    
    # Force kill if still running (ignore errors)
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "THRM Core.exe" /T'
    Pop $0
    Pop $1

    # Backward compatibility: stop legacy core service process
    ClearErrors
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "BS2PRO-Core.exe" /T'
    Pop $0
    Pop $1
    ${If} $0 == 0
        DetailPrint "$(THRM_STR_CLOSE_LEGACY_CORE)"
        Sleep 2000
    ${EndIf}

    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "BS2PRO-Core.exe" /T'
    Pop $0
    Pop $1

    # Stop conflicting SpaceStation service process
    ClearErrors
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "SpaceStationService.exe" /T'
    Pop $0
    Pop $1
    ${If} $0 == 0
        DetailPrint "$(THRM_STR_CLOSE_SPACESTATION)"
        Sleep 1000
    ${EndIf}

    # Force kill if still running (ignore errors)
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "SpaceStationService.exe" /T'
    Pop $0
    Pop $1
    
    # Try to stop the main application gracefully first
    ClearErrors
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "${PRODUCT_EXECUTABLE}" /T'
    Pop $0
    Pop $1
    ${If} $0 == 0
        DetailPrint "$(THRM_STR_CLOSE_APP)"
        Sleep 2000
    ${EndIf}
    
    # Force kill if still running (ignore errors)
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "${PRODUCT_EXECUTABLE}" /T'
    Pop $0
    Pop $1

    # Backward compatibility: kill legacy main executable names
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "BS2PRO-Controller.exe" /T'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "BS2PRO-controller.exe" /T'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "BS2PRO.exe" /T'
    Pop $0
    Pop $1
    
    # Stop any bridge processes (ignore errors)
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "THRM TempBridge.exe" /T'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "TempBridge.exe" /T'
    Pop $0
    Pop $1
    
    # Remove scheduled task if exists (ignore errors)
    DetailPrint "$(THRM_STR_CLEAN_TASKS)"
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "THRM" /f'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "BS2PRO-Controller" /f'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "BS2PRO-Core" /f'
    Pop $0
    Pop $1
    
    # Wait a moment for processes to fully terminate
    DetailPrint "$(THRM_STR_WAIT_TERMINATE)"
    Sleep 2000
    
    DetailPrint "$(THRM_STR_PROCESS_DONE)"
FunctionEnd

# Function to backup user data before upgrade
Function BackupUserData
    DetailPrint "$(THRM_STR_BACKUP_CONFIG)"
    
    # Backup configuration files if they exist
    ${If} ${FileExists} "$INSTDIR\config.json"
        CopyFiles "$INSTDIR\config.json" "$TEMP\bs2pro_config_backup.json"
        DetailPrint "$(THRM_STR_BACKUP_CONFIG_DONE)"
    ${EndIf}
    
    # Backup other important user files if needed
    ${If} ${FileExists} "$INSTDIR\settings.ini"
        CopyFiles "$INSTDIR\settings.ini" "$TEMP\bs2pro_settings_backup.ini"
        DetailPrint "$(THRM_STR_BACKUP_SETTINGS_DONE)"
    ${EndIf}
FunctionEnd

# Function to restore user data after upgrade
Function RestoreUserData
    DetailPrint "$(THRM_STR_RESTORE_CONFIG)"
    
    # Restore configuration files if backup exists
    ${If} ${FileExists} "$TEMP\bs2pro_config_backup.json"
        CopyFiles "$TEMP\bs2pro_config_backup.json" "$INSTDIR\config.json"
        DetailPrint "$(THRM_STR_RESTORE_CONFIG_DONE)"
    ${EndIf}
    
    ${If} ${FileExists} "$TEMP\bs2pro_settings_backup.ini"
        CopyFiles "$TEMP\bs2pro_settings_backup.ini" "$INSTDIR\settings.ini"
        Delete "$TEMP\bs2pro_settings_backup.ini"  # Clean up backup
        DetailPrint "$(THRM_STR_RESTORE_SETTINGS_DONE)"
    ${EndIf}
FunctionEnd

Function CleanupLegacyShortcuts
    DetailPrint "$(THRM_STR_CLEAN_SHORTCUTS)"

    Delete "$SMPROGRAMS\BS2PRO-Controller.lnk"
    Delete "$SMPROGRAMS\BS2PRO-controller.lnk"
    Delete "$SMPROGRAMS\BS2PRO.lnk"
    Delete "$SMPROGRAMS\BS2Pro Controller.lnk"
    Delete "$SMPROGRAMS\BS2PRO Core.lnk"
    Delete "$SMPROGRAMS\BS2PRO-Core.lnk"

    Delete "$DESKTOP\BS2PRO-Controller.lnk"
    Delete "$DESKTOP\BS2PRO-controller.lnk"
    Delete "$DESKTOP\BS2PRO.lnk"
    Delete "$DESKTOP\BS2Pro Controller.lnk"
    Delete "$DESKTOP\BS2PRO Core.lnk"
    Delete "$DESKTOP\BS2PRO-Core.lnk"

    Delete "$SMSTARTUP\BS2PRO-Controller.lnk"
    Delete "$SMSTARTUP\BS2PRO-controller.lnk"
    Delete "$SMSTARTUP\BS2PRO-Core.lnk"
    Delete "$SMSTARTUP\BS2PRO Core.lnk"
FunctionEnd

Function un.CleanupLegacyShortcuts
    DetailPrint "$(THRM_STR_CLEAN_SHORTCUTS)"

    Delete "$SMPROGRAMS\BS2PRO-Controller.lnk"
    Delete "$SMPROGRAMS\BS2PRO-controller.lnk"
    Delete "$SMPROGRAMS\BS2PRO.lnk"
    Delete "$SMPROGRAMS\BS2Pro Controller.lnk"
    Delete "$SMPROGRAMS\BS2PRO Core.lnk"
    Delete "$SMPROGRAMS\BS2PRO-Core.lnk"

    Delete "$DESKTOP\BS2PRO-Controller.lnk"
    Delete "$DESKTOP\BS2PRO-controller.lnk"
    Delete "$DESKTOP\BS2PRO.lnk"
    Delete "$DESKTOP\BS2Pro Controller.lnk"
    Delete "$DESKTOP\BS2PRO Core.lnk"
    Delete "$DESKTOP\BS2PRO-Core.lnk"

    Delete "$SMSTARTUP\BS2PRO-Controller.lnk"
    Delete "$SMSTARTUP\BS2PRO-controller.lnk"
    Delete "$SMSTARTUP\BS2PRO-Core.lnk"
    Delete "$SMSTARTUP\BS2PRO Core.lnk"
FunctionEnd

Section "$(THRM_STR_SECTION_MAIN)" SEC_MAIN
    SectionIn RO  # Read-only, cannot be deselected
    !insertmacro wails.setShellContext

    StrCpy $0 "0"

    # Check if this is an upgrade installation
    ${If} ${FileExists} "$INSTDIR\${PRODUCT_EXECUTABLE}"
        StrCpy $0 "1"
        DetailPrint "$(THRM_STR_UPGRADING) $INSTDIR"
    ${ElseIf} ${FileExists} "$INSTDIR\THRM Core.exe"
        StrCpy $0 "1"
        DetailPrint "$(THRM_STR_UPGRADING) $INSTDIR"
    ${ElseIf} ${FileExists} "$INSTDIR\BS2PRO-Controller.exe"
        StrCpy $0 "1"
        StrCpy $LegacyRenameNoticeNeeded "1"
        DetailPrint "$(THRM_STR_UPGRADING) $INSTDIR"
    ${ElseIf} ${FileExists} "$INSTDIR\BS2PRO-controller.exe"
        StrCpy $0 "1"
        StrCpy $LegacyRenameNoticeNeeded "1"
        DetailPrint "$(THRM_STR_UPGRADING) $INSTDIR"
    ${ElseIf} ${FileExists} "$INSTDIR\BS2PRO.exe"
        StrCpy $0 "1"
        StrCpy $LegacyRenameNoticeNeeded "1"
        DetailPrint "$(THRM_STR_UPGRADING) $INSTDIR"
    ${ElseIf} ${FileExists} "$INSTDIR\BS2PRO-Core.exe"
        StrCpy $0 "1"
        StrCpy $LegacyRenameNoticeNeeded "1"
        DetailPrint "$(THRM_STR_UPGRADING) $INSTDIR"
    ${ElseIf} ${FileExists} "$INSTDIR\uninstall.exe"
        StrCpy $0 "1"
        DetailPrint "$(THRM_STR_UPGRADING) $INSTDIR"
    ${ElseIf} $LegacyRenameNoticeNeeded == "1"
        StrCpy $0 "1"
        DetailPrint "$(THRM_STR_UPGRADING) $INSTDIR"
    ${EndIf}

    ${If} $0 == "1"
        # Backup important files before upgrade
        Call BackupUserData

        # Ensure old instances are completely stopped before upgrading
        Call StopRunningInstances

        # Clean up old files but preserve user data
        DetailPrint "$(THRM_STR_CLEAN_OLD_FILES)"
        Delete "$INSTDIR\${PRODUCT_EXECUTABLE}"
        Delete "$INSTDIR\THRM Core.exe"
        Delete "$INSTDIR\BS2PRO-Controller.exe"
        Delete "$INSTDIR\BS2PRO-controller.exe"
        Delete "$INSTDIR\BS2PRO.exe"
        Delete "$INSTDIR\BS2PRO-Core.exe"
        RMDir /r "$INSTDIR\bridge"
        Delete "$INSTDIR\logs\*.log"  # Keep log structure but remove old logs
    ${Else}
        DetailPrint "$(THRM_STR_FRESH_INSTALL) $INSTDIR"
        
        # Ensure old instances are completely stopped before installing
        Call StopRunningInstances
        
        # Clean up any leftover files from previous installation
        DetailPrint "$(THRM_STR_CLEAN_LEFTOVERS)"
        RMDir /r "$INSTDIR\bridge"
        Delete "$INSTDIR\logs\*.*"
    ${EndIf}
    
    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    !insertmacro wails.files
    
    # Copy core service executable
    DetailPrint "$(THRM_STR_INSTALLING_CORE)"
    File "/oname=THRM Core.exe" "${CORE_EXECUTABLE_SOURCE}"
    
    # Copy bridge directory and its contents
    DetailPrint "$(THRM_STR_INSTALLING_BRIDGE)"
    SetOutPath $INSTDIR\bridge
    File /r "..\..\bin\bridge\*.*"
    
    # Return to main install directory
    SetOutPath $INSTDIR
    
    # Restore user data if this was an upgrade
    Call RestoreUserData

    # Remove legacy shortcuts left by BS2PRO upgrades before creating the new ones
    Call CleanupLegacyShortcuts

    # Create shortcuts
    DetailPrint "$(THRM_STR_CREATING_SHORTCUTS)"
    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller
    Call WriteCurrentVersionInfo
    
    DetailPrint "$(THRM_STR_INSTALL_COMPLETE)"

    ${If} $LegacyRenameNoticeNeeded == "1"
        DetailPrint "$(THRM_STR_UPGRADE_RENAME_DONE)"
    ${ElseIf} ${FileExists} "$TEMP\bs2pro_config_backup.json"
        DetailPrint "$(THRM_STR_UPGRADE_SETTINGS_DONE)"
    ${Else}
        DetailPrint "$(THRM_STR_INSTALL_SUCCESS)"
    ${EndIf}

    ${If} ${FileExists} "$TEMP\bs2pro_config_backup.json"
        Delete "$TEMP\bs2pro_config_backup.json"  # Clean up backup
    ${EndIf}
SectionEnd

# Auto-start section (selected by default)
Section "$(THRM_STR_SECTION_AUTOSTART)" SEC_AUTOSTART
    DetailPrint "$(THRM_STR_CONFIG_AUTOSTART)"
    
    # First, remove any existing auto-start entries to ensure clean state
    DetailPrint "$(THRM_STR_CLEAN_AUTOSTART)"
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "THRM" /f'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "BS2PRO-Controller" /f'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "BS2PRO-Core" /f'
    Pop $0
    Pop $1
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "BS2PRO-Controller"
    DeleteRegValue HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "BS2PRO-Controller"
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "BS2PRO-Core"
    DeleteRegValue HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "BS2PRO-Core"
    
    # Create new scheduled task for auto-start with admin privileges
    DetailPrint "$(THRM_STR_CREATE_AUTOSTART_TASK)"
    
    # Let the core binary register the task from its own XML definition.
    #
    # Do NOT create the task with the schtasks command line here. Without /xml the task
    # inherits three Task Scheduler defaults that are all wrong for a resident fan
    # controller: DisallowStartIfOnBatteries (core never starts when booting on battery),
    # StopIfGoingOnBatteries (unplugging the charger terminates core.exe outright) and
    # ExecutionTimeLimit=PT72H (core is killed after 3 days of uptime). The task also gets
    # no RestartOnFailure, so a crash is never recovered until the next logon.
    #
    # The core's autostart.buildScheduledTaskXML sets all of these correctly, so it is the
    # single source of truth. Its EnsureAutoStartTaskHealthy can repair the task in place,
    # but it cannot be relied upon here: the core is started *by* that task, so a task with
    # DisallowStartIfOnBatteries never fires when the machine boots on battery, the core
    # never runs, and the task is therefore never repaired -- a self-locking loop on exactly
    # the machines this app targets. Registering it correctly up front avoids that entirely.
    #
    # This installer is already elevated (RequestExecutionLevel admin) and the child process
    # inherits that token, so no extra UAC prompt appears.
    nsExec::ExecToStack '"$INSTDIR\THRM Core.exe" --install-autostart'
    Pop $0
    Pop $1
    ${If} $0 == 0
        DetailPrint "$(THRM_STR_AUTOSTART_TASK_OK)"
    ${Else}
        DetailPrint "$(THRM_STR_AUTOSTART_TASK_FAIL)"
        # Fallback: use registry auto-start (will trigger UAC on each login)
        WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "THRM" '"$INSTDIR\THRM Core.exe" --autostart'
        DetailPrint "$(THRM_STR_AUTOSTART_REG_OK)"
    ${EndIf}
SectionEnd

# Required PawnIO installer section
Section "$(THRM_STR_SECTION_PAWNIO)" SEC_PAWNIO
    SectionIn RO
    DetailPrint "$(THRM_STR_PREPARE_PAWNIO)"
    Push $6
    Push $7
    Push $8
    Push $9

    SetOutPath "$INSTDIR\drivers\PawnIO"
    Delete "$INSTDIR\drivers\PawnIO\PawnIO_setup.exe"
    File /nonfatal "..\..\bin\PawnIO_setup.exe"
    StrCpy $7 "$INSTDIR\drivers\PawnIO\PawnIO_setup.exe"
    ${IfNot} ${FileExists} "$7"
        MessageBox MB_OK|MB_ICONSTOP "$(THRM_STR_PAWNIO_MISSING)"
        Abort
    ${EndIf}

    # Detect installed PawnIO version
    StrCpy $6 ""
    SetRegView 64
    ReadRegStr $6 HKLM "SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO" "DisplayVersion"
    ${If} $6 == ""
        SetRegView 32
        ReadRegStr $6 HKLM "SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO" "DisplayVersion"
    ${EndIf}
    SetRegView 64

    # Decide install strategy:
    # $9 = 0 skip, 1 install/update without uninstalling the shared driver first
    StrCpy $9 "1"

    ${If} $6 != ""
        DetailPrint "$(THRM_STR_PAWNIO_DETECTED) $6, $(THRM_STR_PAWNIO_BUNDLED) ${PAWNIO_BUNDLED_VERSION}"
        ${VersionCompare} "$6" "${PAWNIO_BUNDLED_VERSION}" $8

        ${If} $8 == 2
            DetailPrint "$(THRM_STR_PAWNIO_UPDATE)"
            StrCpy $9 "1"
        ${Else}
            DetailPrint "$(THRM_STR_PAWNIO_SKIP)"
            StrCpy $9 "0"
        ${EndIf}
    ${EndIf}

    ${If} $9 == "0"
        DetailPrint "$(THRM_STR_PAWNIO_SKIP_DONE)"
        Goto pawnio_done
    ${EndIf}

    DetailPrint "$(THRM_STR_PAWNIO_SILENT)"
    nsExec::ExecToStack /TIMEOUT=60000 '"$7" -install -silent'
    Pop $0
    Pop $1
    ${If} $0 == "timeout"
        DetailPrint "$(THRM_STR_PAWNIO_TIMEOUT)"
        nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "PawnIO_setup.exe" /T'
        Pop $2
        Pop $3
        ExecWait '"$7" -install' $0
        ${If} $0 == 0
            DetailPrint "$(THRM_STR_PAWNIO_INTERACTIVE_OK)"
        ${Else}
            MessageBox MB_OK|MB_ICONSTOP "$(THRM_STR_PAWNIO_INTERACTIVE_FAIL)"
            Abort
        ${EndIf}
    ${ElseIf} $0 == 0
        DetailPrint "$(THRM_STR_PAWNIO_SILENT_OK)"
    ${Else}
        DetailPrint "$(THRM_STR_PAWNIO_FALLBACK)"
        ExecWait '"$7" -install' $0
        ${If} $0 == 0
            DetailPrint "$(THRM_STR_PAWNIO_INTERACTIVE_OK)"
        ${Else}
            MessageBox MB_OK|MB_ICONSTOP "$(THRM_STR_PAWNIO_FAIL)"
            Abort
        ${EndIf}
    ${EndIf}

    pawnio_done:
    Pop $9
    Pop $8
    Pop $7
    Pop $6
SectionEnd

# Section descriptions
!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
    !insertmacro MUI_DESCRIPTION_TEXT ${SEC_MAIN} "$(THRM_STR_DESC_MAIN)"
    !insertmacro MUI_DESCRIPTION_TEXT ${SEC_AUTOSTART} "$(THRM_STR_DESC_AUTOSTART)"
    !insertmacro MUI_DESCRIPTION_TEXT ${SEC_PAWNIO} "$(THRM_STR_DESC_PAWNIO)"
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "uninstall"
    !insertmacro wails.setShellContext

    # Stop running instances before uninstalling
    DetailPrint "$(THRM_STR_UNINSTALL_STOP)"
    
    # Stop core service first (ignore errors)
    DetailPrint "$(THRM_STR_STOP_CORE)"
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "THRM Core.exe" /T'
    Pop $0
    Pop $1
    Sleep 1000
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "THRM Core.exe" /T'
    Pop $0
    Pop $1

    DetailPrint "$(THRM_STR_STOP_LEGACY_CORE)"
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "BS2PRO-Core.exe" /T'
    Pop $0
    Pop $1
    Sleep 1000
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "BS2PRO-Core.exe" /T'
    Pop $0
    Pop $1
    
    # Stop main application (ignore errors)
    DetailPrint "$(THRM_STR_STOP_APP)"
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "${PRODUCT_EXECUTABLE}" /T'
    Pop $0
    Pop $1
    Sleep 1000
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "${PRODUCT_EXECUTABLE}" /T'
    Pop $0
    Pop $1

    # Backward compatibility: stop legacy main executable names
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "BS2PRO-Controller.exe" /T'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "BS2PRO-controller.exe" /T'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "BS2PRO.exe" /T'
    Pop $0
    Pop $1
    
    # Stop bridge processes (ignore errors)
    DetailPrint "$(THRM_STR_STOP_BRIDGE)"
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "THRM TempBridge.exe" /T'
    Pop $0
    Pop $1
    Sleep 500
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "THRM TempBridge.exe" /T'
    Pop $0
    Pop $1

    DetailPrint "$(THRM_STR_STOP_LEGACY_BRIDGE)"
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "TempBridge.exe" /T'
    Pop $0
    Pop $1
    Sleep 500
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "TempBridge.exe" /T'
    Pop $0
    Pop $1

    # PawnIO owns the shared R0 driver lifecycle; do not stop/delete it from THRM uninstall.
    
    # Remove auto-start entries
    DetailPrint "$(THRM_STR_REMOVE_AUTOSTART)"
    
    # Remove scheduled task (ignore errors if not exists)
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "THRM" /f'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "BS2PRO-Controller" /f'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "BS2PRO-Core" /f'
    Pop $0
    Pop $1
    
    # Remove registry auto-start entry (both current user and local machine)
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "BS2PRO-Controller"
    DeleteRegValue HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "BS2PRO-Controller"
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "BS2PRO-Core"
    DeleteRegValue HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "BS2PRO-Core"
    
    # Remove from startup folder if exists
    Delete "$SMSTARTUP\THRM.lnk"
    Delete "$SMSTARTUP\BS2PRO-Core.lnk"
    
    # Wait for processes to fully terminate
    Sleep 2000

    # Remove application data directories
    DetailPrint "$(THRM_STR_REMOVE_APPDATA)"
    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath
    RMDir /r "$APPDATA\THRM"
    RMDir /r "$LOCALAPPDATA\THRM"
    RMDir /r "$TEMP\THRM"

    # Remove installation directory and all contents
    DetailPrint "$(THRM_STR_REMOVE_INSTALL_FILES)"
    
    # Remove bridge directory (contains THRM TempBridge.exe and related files)
    DetailPrint "$(THRM_STR_REMOVE_BRIDGE)"
    RMDir /r "$INSTDIR\bridge"
    
    # Remove logs directory
    DetailPrint "$(THRM_STR_REMOVE_LOGS)"
    RMDir /r "$INSTDIR\logs"
    
    # Remove entire installation directory
    DetailPrint "$(THRM_STR_REMOVE_DIR)"
    RMDir /r $INSTDIR

    # Remove shortcuts
    DetailPrint "$(THRM_STR_REMOVE_SHORTCUTS)"
    Call un.CleanupLegacyShortcuts
    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"
    Delete "$SMSTARTUP\THRM.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
    
    DetailPrint "$(THRM_STR_UNINSTALL_COMPLETE)"
    
    # Optional: Ask user if they want to remove configuration files
    MessageBox MB_YESNO|MB_ICONQUESTION "$(THRM_STR_UNINSTALL_REMOVE_CONFIG)" IDNO skip_config
    RMDir /r "$APPDATA\BS2PRO"
    RMDir /r "$LOCALAPPDATA\BS2PRO"
    skip_config:
SectionEnd
