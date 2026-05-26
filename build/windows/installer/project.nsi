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

!macro TryInstallDirCandidate CANDIDATE SOURCE LEGACY
    ${If} "${CANDIDATE}" != ""
        ${If} ${FileExists} "${CANDIDATE}\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR "${CANDIDATE}"
            DetailPrint "发现已有安装 (${SOURCE}-主程序): $INSTDIR"
            ${If} "${LEGACY}" == "1"
                Goto found_legacy_installation
            ${Else}
                Goto found_installation
            ${EndIf}
        ${EndIf}
        ${If} ${FileExists} "${CANDIDATE}\THRM Core.exe"
            StrCpy $INSTDIR "${CANDIDATE}"
            DetailPrint "发现已有安装 (${SOURCE}-Core): $INSTDIR"
            ${If} "${LEGACY}" == "1"
                Goto found_legacy_installation
            ${Else}
                Goto found_installation
            ${EndIf}
        ${EndIf}
        ${If} ${FileExists} "${CANDIDATE}\BS2PRO-Controller.exe"
            StrCpy $INSTDIR "${CANDIDATE}"
            DetailPrint "发现旧版安装 (${SOURCE}-旧主程序): $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "${CANDIDATE}\BS2PRO-controller.exe"
            StrCpy $INSTDIR "${CANDIDATE}"
            DetailPrint "发现旧版安装 (${SOURCE}-旧主程序): $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "${CANDIDATE}\BS2PRO.exe"
            StrCpy $INSTDIR "${CANDIDATE}"
            DetailPrint "发现旧版安装 (${SOURCE}-旧主程序): $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "${CANDIDATE}\BS2PRO-Core.exe"
            StrCpy $INSTDIR "${CANDIDATE}"
            DetailPrint "发现旧版安装 (${SOURCE}-旧 Core): $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "${CANDIDATE}\uninstall.exe"
            StrCpy $INSTDIR "${CANDIDATE}"
            DetailPrint "发现已有安装 (${SOURCE}-卸载器): $INSTDIR"
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
!define MUI_FINISHPAGE_RUN_TEXT "安装完成后立即启动 THRM"
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!define MUI_PAGE_CUSTOMFUNCTION_PRE WelcomePagePre
!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_COMPONENTS # Component selection page
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uinstalling page

!insertmacro MUI_LANGUAGE "SimpChinese" # Set the Language of the installer

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
Caption "${INFO_PRODUCTNAME} 安装程序 v${INFO_PRODUCTVERSION}"
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
       MessageBox MB_OK|MB_ICONSTOP "需要 .NET Framework 4.7.2 或更高版本。$\n$\n请先安装 .NET Framework 4.7.2。"
       Abort
   ${EndIf}
   
    # Check for existing installation and set install directory
   Call DetectExistingInstallation
FunctionEnd

Function WelcomePagePre
    ${If} $LegacyRenameNoticeNeeded == "1"
        !insertmacro INSTALLOPTIONS_WRITE "ioSpecial.ini" "Field 2" "Text" "安装前提示：BS2Pro Controller 已更名为 THRM"
        !insertmacro INSTALLOPTIONS_WRITE "ioSpecial.ini" "Field 3" "Text" "检测到你正在从旧版 BS2Pro Controller 升级。$\r$\n$\r$\nTHRM 3.0 已正式完成更名，本次安装将继续沿用升级流程：$\r$\n1. 自动保留现有配置和用户数据；$\r$\n2. 默认继续使用当前安装目录，避免升级中断；$\r$\n3. 安装完成后程序名称统一变更为 THRM。$\r$\n$\r$\n如果你希望安装目录也改成 THRM，请在下一步“安装位置”页面手动修改。"
    ${EndIf}
FunctionEnd

# Function to clean up legacy/duplicate registry keys
Function CleanLegacyRegistryKeys
    DetailPrint "正在清理历史注册表项..."
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
        DetailPrint "发现重复注册表键: BS2PRO-controllerBS2PRO-controller"
        DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-controllerBS2PRO-controller"
        DetailPrint "已删除重复注册表键"
    ${EndIf}

    # Check and remove BS2PRO-ControllerBS2PRO-Controller
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-ControllerBS2PRO-Controller" "UninstallString"
    ${If} $R0 != ""
        DetailPrint "发现重复注册表键: BS2PRO-ControllerBS2PRO-Controller"
        DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-ControllerBS2PRO-Controller"
        DetailPrint "已删除重复注册表键"
    ${EndIf}

    # Check and remove BS2PRO-Controller (actual legacy product key)
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-Controller" "UninstallString"
    ${If} $R0 != ""
        DetailPrint "发现旧版注册表键: BS2PRO-Controller"
        DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\BS2PRO-Controller"
        DetailPrint "已删除旧版注册表键"
    ${EndIf}
    
    # Check and remove TIANLI0BS2PRO-Controller
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO-Controller" "UninstallString"
    ${If} $R0 != ""
        DetailPrint "发现旧版注册表键: TIANLI0BS2PRO-Controller"
        DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO-Controller"
        DetailPrint "已删除旧版注册表键"
    ${EndIf}
    
    # Check and remove TIANLI0BS2PRO (current wails.json would generate this)
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO" "UninstallString"
    ${If} $R0 != ""
        DetailPrint "发现重复注册表键: TIANLI0BS2PRO"
        DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO"
        DetailPrint "已删除重复注册表键"
    ${EndIf}
    
    Pop $R1
    Pop $R0
FunctionEnd

# Function to detect existing installation and set install directory
Function DetectExistingInstallation
    DetailPrint "正在检查已有安装..."
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
        DetailPrint "本地已安装版本: $R2"
    ${Else}
        DetailPrint "本地未检测到已安装版本信息"
    ${EndIf}
    
    # First, check all possible registry keys to find installation path
    # DO NOT delete registry keys yet - we need them to find the install path!
    
    # Method 1: Try current registry key (THRM)
    ReadRegStr $R0 HKLM "${UNINST_KEY}" "InstallLocation"
    !insertmacro TryInstallDirCandidate "$R0" "正确键-安装位置" "0"
    ${If} $R0 != ""
        ${If} ${FileExists} "$R0\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R0
            DetailPrint "发现已有安装 (正确键-安装位置): $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R0\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R0
            DetailPrint "发现已有安装 (正确键-安装位置-Core): $INSTDIR"
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
            DetailPrint "发现已有安装 (从正确的注册表键): $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "发现已有安装 (从正确的注册表键-Core): $INSTDIR"
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
            DetailPrint "发现旧版安装 (重复键-安装位置): $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "$R0\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R0
            DetailPrint "发现旧版安装 (重复键-安装位置-Core): $INSTDIR"
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
            DetailPrint "发现旧版安装 (重复键): $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "发现旧版安装 (重复键-Core): $INSTDIR"
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
            DetailPrint "发现旧版安装 (从图标路径): $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "发现旧版安装 (从图标路径-Core): $INSTDIR"
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
            DetailPrint "发现旧版安装 (旧格式键-安装位置): $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "$R0\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R0
            DetailPrint "发现旧版安装 (旧格式键-安装位置-Core): $INSTDIR"
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
            DetailPrint "发现旧版安装 (旧格式键): $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "发现旧版安装 (旧格式键-Core): $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
    ${EndIf}
    
    # Method 5: Check TIANLI0BS2PRO (wails.json generates this)
    ReadRegStr $R0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\TIANLI0BS2PRO" "InstallLocation"
    !insertmacro TryInstallDirCandidate "$R0" "TIANLI0BS2PRO-安装位置" "1"
    ${If} $R0 != ""
        ${If} ${FileExists} "$R0\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R0
            DetailPrint "发现旧版安装 (TIANLI0BS2PRO-安装位置): $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "$R0\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R0
            DetailPrint "发现旧版安装 (TIANLI0BS2PRO-安装位置-Core): $INSTDIR"
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
            DetailPrint "发现旧版安装 (TIANLI0BS2PRO): $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "发现旧版安装 (TIANLI0BS2PRO-Core): $INSTDIR"
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
            DetailPrint "发现已有安装 (从图标): $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R1\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R1
            DetailPrint "发现已有安装 (从图标-Core): $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
    ${EndIf}
    
    # Third, try to read InstallLocation from registry
    ReadRegStr $R0 HKLM "${UNINST_KEY}" "InstallLocation"
    !insertmacro TryInstallDirCandidate "$R0" "安装位置" "0"
    ${If} $R0 != ""
        ${If} ${FileExists} "$R0\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR $R0
            DetailPrint "发现已有安装 (从安装位置): $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "$R0\BS2PRO-Core.exe"
            StrCpy $INSTDIR $R0
            DetailPrint "发现已有安装 (从安装位置-Core): $INSTDIR"
            Goto found_legacy_installation
        ${EndIf}
    ${EndIf}
    
    # Fourth, check common installation locations (single level path)
    ${If} ${FileExists} "$PROGRAMFILES64\${INFO_PRODUCTNAME}\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES64\${INFO_PRODUCTNAME}"
        DetailPrint "发现已有安装: $INSTDIR"
        Goto found_installation
    ${EndIf}
    
    ${If} ${FileExists} "$PROGRAMFILES32\${INFO_PRODUCTNAME}\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES32\${INFO_PRODUCTNAME}"
        DetailPrint "发现已有安装: $INSTDIR"
        Goto found_installation
    ${EndIf}
    
    # Fifth, check legacy paths with Company\Product structure
    ${If} ${FileExists} "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
        DetailPrint "发现已有安装 (旧版路径): $INSTDIR"
        Goto found_installation
    ${EndIf}

    !insertmacro TryInstallDirCandidate "$PROGRAMFILES64\TIANLI0\BS2PRO-Controller" "旧公司目录" "1"
    !insertmacro TryInstallDirCandidate "$PROGRAMFILES32\TIANLI0\BS2PRO-Controller" "旧公司目录" "1"
    !insertmacro TryInstallDirCandidate "$PROGRAMFILES64\BS2PRO-controller" "旧目录" "1"
    !insertmacro TryInstallDirCandidate "$PROGRAMFILES32\BS2PRO-controller" "旧目录" "1"
    
    # Sixth, try alternative common paths
    ${If} ${FileExists} "$PROGRAMFILES64\THRM\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES64\THRM"
        DetailPrint "发现已有安装: $INSTDIR"
        Goto found_installation
    ${EndIf}
    
    ${If} ${FileExists} "$PROGRAMFILES32\THRM\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES32\THRM"
        DetailPrint "发现已有安装: $INSTDIR"
        Goto found_installation
    ${EndIf}
    
    # Seventh, check for THRM Core.exe in common paths
    ${If} ${FileExists} "$PROGRAMFILES64\${INFO_PRODUCTNAME}\THRM Core.exe"
        StrCpy $INSTDIR "$PROGRAMFILES64\${INFO_PRODUCTNAME}"
        DetailPrint "发现已有安装 (Core): $INSTDIR"
        Goto found_installation
    ${EndIf}
    
    ${If} ${FileExists} "$PROGRAMFILES64\THRM\THRM Core.exe"
        StrCpy $INSTDIR "$PROGRAMFILES64\THRM"
        DetailPrint "发现已有安装 (Core): $INSTDIR"
        Goto found_installation
    ${EndIf}

    ${If} ${FileExists} "$PROGRAMFILES64\BS2PRO-Controller\BS2PRO-Controller.exe"
        StrCpy $INSTDIR "$PROGRAMFILES64\BS2PRO-Controller"
        DetailPrint "发现旧版安装 (旧目录-主程序): $INSTDIR"
        Goto found_legacy_installation
    ${EndIf}

    ${If} ${FileExists} "$PROGRAMFILES32\BS2PRO-Controller\BS2PRO-Controller.exe"
        StrCpy $INSTDIR "$PROGRAMFILES32\BS2PRO-Controller"
        DetailPrint "发现旧版安装 (旧目录-主程序): $INSTDIR"
        Goto found_legacy_installation
    ${EndIf}

    ${If} ${FileExists} "$PROGRAMFILES64\BS2PRO-Controller\BS2PRO-Core.exe"
        StrCpy $INSTDIR "$PROGRAMFILES64\BS2PRO-Controller"
        DetailPrint "发现旧版安装 (旧目录-Core): $INSTDIR"
        Goto found_legacy_installation
    ${EndIf}

    ${If} ${FileExists} "$PROGRAMFILES32\BS2PRO-Controller\BS2PRO-Core.exe"
        StrCpy $INSTDIR "$PROGRAMFILES32\BS2PRO-Controller"
        DetailPrint "发现旧版安装 (旧目录-Core): $INSTDIR"
        Goto found_legacy_installation
    ${EndIf}
    
    # If no existing installation found, use simple product name for directory
    # Use THRM as the default install directory
    StrCpy $INSTDIR "$PROGRAMFILES64\THRM"
    DetailPrint "未发现已有安装,使用默认目录: $INSTDIR"
    Goto end_detection

    found_legacy_installation:
    StrCpy $LegacyRenameNoticeNeeded "1"
    Goto found_installation
    
    found_installation:
    DetailPrint "检测到已有安装 - 将执行升级到: $INSTDIR"
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
    DetailPrint "已写入版本信息: ${INFO_PRODUCTVERSION}"
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
    DetailPrint "正在检查运行中的进程..."
    
    # Try to stop the core service first (it manages the fan control)
    # Use /FI with proper error handling
    ClearErrors
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "THRM Core.exe" /T'
    Pop $0
    Pop $1
    ${If} $0 == 0
        DetailPrint "已请求关闭 THRM Core.exe..."
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
        DetailPrint "已请求关闭 BS2PRO-Core.exe..."
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
        DetailPrint "已请求关闭 SpaceStationService.exe..."
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
        DetailPrint "已请求关闭 ${PRODUCT_EXECUTABLE}..."
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
    DetailPrint "正在清理计划任务..."
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
    DetailPrint "等待进程完全终止..."
    Sleep 2000
    
    DetailPrint "进程清理完成"
FunctionEnd

# Function to backup user data before upgrade
Function BackupUserData
    DetailPrint "正在备份用户配置..."
    
    # Backup configuration files if they exist
    ${If} ${FileExists} "$INSTDIR\config.json"
        CopyFiles "$INSTDIR\config.json" "$TEMP\bs2pro_config_backup.json"
        DetailPrint "配置文件已备份"
    ${EndIf}
    
    # Backup other important user files if needed
    ${If} ${FileExists} "$INSTDIR\settings.ini"
        CopyFiles "$INSTDIR\settings.ini" "$TEMP\bs2pro_settings_backup.ini"
        DetailPrint "设置文件已备份"
    ${EndIf}
FunctionEnd

# Function to restore user data after upgrade
Function RestoreUserData
    DetailPrint "正在恢复用户配置..."
    
    # Restore configuration files if backup exists
    ${If} ${FileExists} "$TEMP\bs2pro_config_backup.json"
        CopyFiles "$TEMP\bs2pro_config_backup.json" "$INSTDIR\config.json"
        DetailPrint "配置文件已恢复"
    ${EndIf}
    
    ${If} ${FileExists} "$TEMP\bs2pro_settings_backup.ini"
        CopyFiles "$TEMP\bs2pro_settings_backup.ini" "$INSTDIR\settings.ini"
        Delete "$TEMP\bs2pro_settings_backup.ini"  # Clean up backup
        DetailPrint "设置文件已恢复"
    ${EndIf}
FunctionEnd

Section "主程序 (必需)" SEC_MAIN
    SectionIn RO  # Read-only, cannot be deselected
    !insertmacro wails.setShellContext

    StrCpy $0 "0"

    # Check if this is an upgrade installation
    ${If} ${FileExists} "$INSTDIR\${PRODUCT_EXECUTABLE}"
        StrCpy $0 "1"
        DetailPrint "正在升级: $INSTDIR"
    ${ElseIf} ${FileExists} "$INSTDIR\THRM Core.exe"
        StrCpy $0 "1"
        DetailPrint "正在升级 (发现 THRM Core): $INSTDIR"
    ${ElseIf} ${FileExists} "$INSTDIR\BS2PRO-Controller.exe"
        StrCpy $0 "1"
        StrCpy $LegacyRenameNoticeNeeded "1"
        DetailPrint "正在升级 (发现旧版主程序): $INSTDIR"
    ${ElseIf} ${FileExists} "$INSTDIR\BS2PRO-controller.exe"
        StrCpy $0 "1"
        StrCpy $LegacyRenameNoticeNeeded "1"
        DetailPrint "正在升级 (发现旧版主程序): $INSTDIR"
    ${ElseIf} ${FileExists} "$INSTDIR\BS2PRO.exe"
        StrCpy $0 "1"
        StrCpy $LegacyRenameNoticeNeeded "1"
        DetailPrint "正在升级 (发现旧版主程序): $INSTDIR"
    ${ElseIf} ${FileExists} "$INSTDIR\BS2PRO-Core.exe"
        StrCpy $0 "1"
        StrCpy $LegacyRenameNoticeNeeded "1"
        DetailPrint "正在升级 (发现旧版 Core): $INSTDIR"
    ${ElseIf} ${FileExists} "$INSTDIR\uninstall.exe"
        StrCpy $0 "1"
        DetailPrint "正在升级 (发现卸载器): $INSTDIR"
    ${ElseIf} $LegacyRenameNoticeNeeded == "1"
        StrCpy $0 "1"
        DetailPrint "正在升级 (沿用旧版安装目录): $INSTDIR"
    ${EndIf}

    ${If} $0 == "1"
        # Backup important files before upgrade
        Call BackupUserData

        # Ensure old instances are completely stopped before upgrading
        Call StopRunningInstances

        # Clean up old files but preserve user data
        DetailPrint "正在清理旧版本文件..."
        Delete "$INSTDIR\${PRODUCT_EXECUTABLE}"
        Delete "$INSTDIR\THRM Core.exe"
        Delete "$INSTDIR\BS2PRO-Controller.exe"
        Delete "$INSTDIR\BS2PRO-controller.exe"
        Delete "$INSTDIR\BS2PRO.exe"
        Delete "$INSTDIR\BS2PRO-Core.exe"
        RMDir /r "$INSTDIR\bridge"
        Delete "$INSTDIR\logs\*.log"  # Keep log structure but remove old logs
    ${Else}
        DetailPrint "全新安装: $INSTDIR"
        
        # Ensure old instances are completely stopped before installing
        Call StopRunningInstances
        
        # Clean up any leftover files from previous installation
        DetailPrint "正在清理残留文件..."
        RMDir /r "$INSTDIR\bridge"
        Delete "$INSTDIR\logs\*.*"
    ${EndIf}
    
    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    !insertmacro wails.files
    
    # Copy core service executable
    DetailPrint "正在安装核心服务..."
    File "/oname=THRM Core.exe" "${CORE_EXECUTABLE_SOURCE}"
    
    # Copy bridge directory and its contents
    DetailPrint "正在安装桥接组件..."
    SetOutPath $INSTDIR\bridge
    File /r "..\..\bin\bridge\*.*"
    
    # Return to main install directory
    SetOutPath $INSTDIR
    
    # Restore user data if this was an upgrade
    Call RestoreUserData

    # Create shortcuts
    DetailPrint "正在创建快捷方式..."
    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller
    Call WriteCurrentVersionInfo
    
    DetailPrint "安装完成"

    ${If} $LegacyRenameNoticeNeeded == "1"
        DetailPrint "已完成旧版 BS2Pro Controller 到 THRM 的升级说明与安装。"
    ${ElseIf} ${FileExists} "$TEMP\bs2pro_config_backup.json"
        DetailPrint "已完成升级，原有设置已保留。"
    ${Else}
        DetailPrint "THRM 安装成功。"
    ${EndIf}

    ${If} ${FileExists} "$TEMP\bs2pro_config_backup.json"
        Delete "$TEMP\bs2pro_config_backup.json"  # Clean up backup
    ${EndIf}
SectionEnd

# Auto-start section (selected by default)
Section "开机自启动" SEC_AUTOSTART
    DetailPrint "正在配置开机自启动..."
    
    # First, remove any existing auto-start entries to ensure clean state
    DetailPrint "正在清理现有自启动项..."
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
    DetailPrint "正在创建自启动计划任务..."
    
    # Use schtasks to create a task that runs at logon with highest privileges
    # The task will start THRM Core.exe with --autostart flag after 15 seconds delay
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /create /tn "THRM" /tr "\"$INSTDIR\THRM Core.exe\" --autostart" /sc onlogon /delay 0000:15 /rl highest /f'
    Pop $0
    Pop $1
    ${If} $0 == 0
        DetailPrint "开机自启动配置成功（计划任务）"
    ${Else}
        DetailPrint "计划任务创建失败，使用注册表方式..."
        # Fallback: use registry auto-start (will trigger UAC on each login)
        WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "THRM" '"$INSTDIR\THRM Core.exe" --autostart'
        DetailPrint "开机自启动配置成功（注册表）"
    ${EndIf}
SectionEnd

# Required PawnIO installer section
Section "安装 PawnIO (必需)" SEC_PAWNIO
    SectionIn RO
    DetailPrint "正在准备安装 PawnIO..."
    Push $6
    Push $7
    Push $8
    Push $9

    SetOutPath "$INSTDIR\drivers\PawnIO"
    Delete "$INSTDIR\drivers\PawnIO\PawnIO_setup.exe"
    File /nonfatal "..\..\bin\PawnIO_setup.exe"
    StrCpy $7 "$INSTDIR\drivers\PawnIO\PawnIO_setup.exe"
    ${IfNot} ${FileExists} "$7"
        MessageBox MB_OK|MB_ICONSTOP "未找到 PawnIO_setup.exe（build\\bin）。请先执行 build_bridge.bat 下载后再打包安装器。"
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
        DetailPrint "检测到已安装 PawnIO (版本: $6)，内置版本: ${PAWNIO_BUNDLED_VERSION}"
        ${VersionCompare} "$6" "${PAWNIO_BUNDLED_VERSION}" $8

        ${If} $8 == 2
            DetailPrint "检测到 PawnIO 旧版本，将直接尝试静默更新；不会先卸载共享驱动。"
            StrCpy $9 "1"
        ${Else}
            DetailPrint "PawnIO 已安装且版本满足要求，跳过驱动安装。"
            StrCpy $9 "0"
        ${EndIf}
    ${EndIf}

    pawnio_apply:
    ${If} $9 == "0"
        DetailPrint "跳过 PawnIO 处理。"
        Goto pawnio_done
    ${EndIf}

    DetailPrint "正在静默安装/更新 PawnIO（最多等待 60 秒）..."
    nsExec::ExecToStack /TIMEOUT=60000 '"$7" -install -silent'
    Pop $0
    Pop $1
    ${If} $0 == "timeout"
        DetailPrint "PawnIO 静默安装/更新 60 秒未响应，回退到交互安装..."
        nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "PawnIO_setup.exe" /T'
        Pop $2
        Pop $3
        ExecWait '"$7" -install' $0
        ${If} $0 == 0
            DetailPrint "PawnIO 安装/更新完成（交互）"
        ${Else}
            MessageBox MB_OK|MB_ICONSTOP "PawnIO 交互安装/更新失败（返回码: $0）。$\n$\n常见原因：驱动服务被系统标记删除（错误 1072）。$\n请先重启系统后重新运行安装程序。"
            Abort
        ${EndIf}
    ${ElseIf} $0 == 0
        DetailPrint "PawnIO 安装/更新完成（静默）"
    ${Else}
        DetailPrint "PawnIO 静默安装/更新失败，改为交互安装..."
        ExecWait '"$7" -install' $0
        ${If} $0 == 0
            DetailPrint "PawnIO 安装/更新完成（交互）"
        ${Else}
            MessageBox MB_OK|MB_ICONSTOP "PawnIO 安装/更新失败（返回码: $0）。$\n$\n常见原因：驱动服务被系统标记删除（错误 1072）。$\n请先重启系统后重新运行安装程序。"
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
    !insertmacro MUI_DESCRIPTION_TEXT ${SEC_MAIN} "THRM 主程序和核心服务文件。"
    !insertmacro MUI_DESCRIPTION_TEXT ${SEC_AUTOSTART} "系统启动时自动运行 THRM Core。推荐开启。"
    !insertmacro MUI_DESCRIPTION_TEXT ${SEC_PAWNIO} "安装 PawnIO 驱动，PawnIO将用于获取硬件相关信息。"
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "uninstall"
    !insertmacro wails.setShellContext

    # Stop running instances before uninstalling
    DetailPrint "正在停止运行中的进程..."
    
    # Stop core service first (ignore errors)
    DetailPrint "正在停止 THRM Core.exe..."
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "THRM Core.exe" /T'
    Pop $0
    Pop $1
    Sleep 1000
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "THRM Core.exe" /T'
    Pop $0
    Pop $1

    DetailPrint "正在停止 BS2PRO-Core.exe..."
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "BS2PRO-Core.exe" /T'
    Pop $0
    Pop $1
    Sleep 1000
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "BS2PRO-Core.exe" /T'
    Pop $0
    Pop $1
    
    # Stop main application (ignore errors)
    DetailPrint "正在停止 ${PRODUCT_EXECUTABLE}..."
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
    DetailPrint "正在停止 THRM TempBridge.exe..."
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "THRM TempBridge.exe" /T'
    Pop $0
    Pop $1
    Sleep 500
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "THRM TempBridge.exe" /T'
    Pop $0
    Pop $1

    DetailPrint "正在停止 TempBridge.exe..."
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "TempBridge.exe" /T'
    Pop $0
    Pop $1
    Sleep 500
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "TempBridge.exe" /T'
    Pop $0
    Pop $1

    # PawnIO owns the shared R0 driver lifecycle; do not stop/delete it from THRM uninstall.
    
    # Remove auto-start entries
    DetailPrint "正在移除自启动项..."
    
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
    DetailPrint "正在移除应用数据..."
    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath
    RMDir /r "$APPDATA\THRM"
    RMDir /r "$LOCALAPPDATA\THRM"
    RMDir /r "$TEMP\THRM"

    # Remove installation directory and all contents
    DetailPrint "正在移除安装文件..."
    
    # Remove bridge directory (contains THRM TempBridge.exe and related files)
    DetailPrint "正在删除桥接组件..."
    RMDir /r "$INSTDIR\bridge"
    
    # Remove logs directory
    DetailPrint "正在删除日志文件..."
    RMDir /r "$INSTDIR\logs"
    
    # Remove entire installation directory
    DetailPrint "正在删除安装目录..."
    RMDir /r $INSTDIR

    # Remove shortcuts
    DetailPrint "正在移除快捷方式..."
    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
    
    DetailPrint "卸载完成"
    
    # Optional: Ask user if they want to remove configuration files
    MessageBox MB_YESNO|MB_ICONQUESTION "是否删除所有配置文件和日志？" IDNO skip_config
    RMDir /r "$APPDATA\BS2PRO"
    RMDir /r "$LOCALAPPDATA\BS2PRO"
    skip_config:
SectionEnd
