package runner

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// Uma junção NTFS de diretório não exige o privilégio de criar symlinks.
func createSkillLinkFallback(source, destination string, original error) error {
	if !errors.Is(original, windows.ERROR_PRIVILEGE_NOT_HELD) {
		return original
	}
	abs, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("origem da skill não é um diretório: %s", abs)
	}
	if filepath.VolumeName(abs) == "" || len(abs) < 3 || abs[1] != ':' {
		return fmt.Errorf("junção de skill exige um caminho local: %s", abs)
	}
	if err := os.Mkdir(destination, 0755); err != nil {
		return err
	}
	success := false
	defer func() {
		if !success {
			_ = os.Remove(destination)
		}
	}()
	name, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	sub := utf16.Encode([]rune(`\??\` + abs))
	printName := utf16.Encode([]rune(abs))
	buf := make([]byte, 16+2*(len(sub)+1+len(printName)+1))
	binary.LittleEndian.PutUint32(buf[0:4], windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(buf[4:6], uint16(len(buf)-8))
	binary.LittleEndian.PutUint16(buf[10:12], uint16(2*len(sub)))
	binary.LittleEndian.PutUint16(buf[12:14], uint16(2*(len(sub)+1)))
	binary.LittleEndian.PutUint16(buf[14:16], uint16(2*len(printName)))
	for i, c := range sub {
		binary.LittleEndian.PutUint16(buf[16+2*i:], c)
	}
	for i, c := range printName {
		binary.LittleEndian.PutUint16(buf[16+2*(len(sub)+1+i):], c)
	}
	var returned uint32
	if err := windows.DeviceIoControl(h, windows.FSCTL_SET_REPARSE_POINT, &buf[0], uint32(len(buf)), nil, 0, &returned, nil); err != nil {
		return err
	}
	success = true
	return nil
}
