package runner

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Códigos de saída oficiais do Windows Package Manager (winget-cli,
// src/AppInstallerSharedLib/Public/AppInstallerErrors.h) e do Windows Installer (MSI).
const (
	// MSI / Windows Installer: instalação/atualização concluída; reinicialização pendente ou iniciada.
	WingetExitRebootRequired  uint32 = 3010 // ERROR_SUCCESS_REBOOT_REQUIRED
	WingetExitRebootInitiated uint32 = 1641 // ERROR_SUCCESS_REBOOT_INITIATED

	// AppInstaller CLI HRESULT codes
	WingetErrNoApplicationsFound     uint32 = 0x8A150014 // APPINSTALLER_CLI_ERROR_NO_APPLICATIONS_FOUND
	WingetErrUpdateNotApplicable     uint32 = 0x8A15002B // APPINSTALLER_CLI_ERROR_UPDATE_NOT_APPLICABLE
	WingetErrNoUninstallInfoFound    uint32 = 0x8A15002F // APPINSTALLER_CLI_ERROR_NO_UNINSTALL_INFO_FOUND
	WingetErrPackageAlreadyInstalled uint32 = 0x8A150061 // APPINSTALLER_CLI_ERROR_PACKAGE_ALREADY_INSTALLED
	WingetErrInstallAlreadyInstalled uint32 = 0x8A15010D // APPINSTALLER_CLI_ERROR_INSTALL_ALREADY_INSTALLED
	WingetErrInstallDowngrade        uint32 = 0x8A15010E // APPINSTALLER_CLI_ERROR_INSTALL_DOWNGRADE
)

func matchWingetHRESULT(err error, codes ...uint32) bool {
	if err == nil {
		return false
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		exitCode := uint32(ee.ExitCode())
		for _, code := range codes {
			if exitCode == code {
				return true
			}
		}
	}
	errStr := strings.ToLower(err.Error())
	for _, code := range codes {
		hexCode := fmt.Sprintf("%x", code)
		decCode := fmt.Sprintf("%d", int32(code))
		if strings.Contains(errStr, hexCode) || strings.Contains(errStr, decCode) {
			return true
		}
	}
	return false
}

// isWingetAlreadyInstalled verifica se o erro do winget indica que o pacote já está instalado
// ou em versão igual/superior (sem atualização aplicável).
func isWingetAlreadyInstalled(err error) bool {
	return matchWingetHRESULT(
		err,
		WingetErrUpdateNotApplicable,
		WingetErrPackageAlreadyInstalled,
		WingetErrInstallAlreadyInstalled,
		WingetErrInstallDowngrade,
	)
}

// isWingetRebootRequired verifica se o instalador do winget retornou sucesso com reinicialização pendente (3010 / 1641).
func isWingetRebootRequired(err error) bool {
	if err == nil {
		return false
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code := uint32(ee.ExitCode())
		if code == WingetExitRebootRequired || code == WingetExitRebootInitiated {
			return true
		}
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "exit status 3010") || strings.Contains(errStr, "exit status 1641")
}

// isWingetPackageNotFound verifica se o winget indicou que o pacote não foi encontrado ao desinstalar.
func isWingetPackageNotFound(err error) bool {
	return matchWingetHRESULT(
		err,
		WingetErrNoApplicationsFound,
		WingetErrNoUninstallInfoFound,
	)
}
