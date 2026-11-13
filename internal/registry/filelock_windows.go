package registry

import (
	"errors"
	"io"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func readRegistryFile(path string) ([]byte, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	// Leitores não devem impedir a substituição atômica por um escritor.
	var handle windows.Handle
	err = retryWindowsFileOperation(func() error {
		var openErr error
		handle, openErr = windows.CreateFile(name, windows.GENERIC_READ,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
			nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
		return openErr
	})
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(handle), path)
	defer f.Close()
	return io.ReadAll(f)
}

func lockRegistryFile(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	overlap := new(windows.Overlapped)
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, overlap); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, overlap)
		_ = f.Close()
	}, nil
}

func replaceRegistryFile(source, destination string) error {
	src, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	dst, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	// os.Rename não garante substituição atômica no Windows. MoveFileEx mantém
	// os arquivos no mesmo volume e solicita a conclusão da gravação em disco.
	return retryWindowsFileOperation(func() error {
		return windows.MoveFileEx(src, dst, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
	})
}

// Windows pode manter handles transitórios abertos durante substituições
// (inclusive por antivírus). A espera é limitada, sem remover o destino.
func retryWindowsFileOperation(operation func() error) error {
	deadline := time.Now().Add(2 * time.Second)
	delay := time.Millisecond
	for {
		err := operation()
		if err == nil || (!errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION)) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(delay)
		if delay < 32*time.Millisecond {
			delay *= 2
		}
	}
}
