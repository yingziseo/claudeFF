//go:build windows

package secret

import (
	"errors"
	"golang.org/x/sys/windows"
	"unsafe"
)

type Vault struct{}
type blob struct {
	Size uint32
	Data *byte
}

var crypt = windows.NewLazySystemDLL("crypt32.dll")
var protect = crypt.NewProc("CryptProtectData")
var unprotect = crypt.NewProc("CryptUnprotectData")

func transform(p []byte, proc *windows.LazyProc) ([]byte, error) {
	if len(p) == 0 {
		return nil, nil
	}
	in := blob{uint32(len(p)), &p[0]}
	var out blob
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	if r == 0 {
		return nil, errors.New("Windows 用户级秘密保护失败")
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte{}, unsafe.Slice(out.Data, out.Size)...), nil
}
func (Vault) Protect(b []byte) ([]byte, error)   { return transform(b, protect) }
func (Vault) Unprotect(b []byte) ([]byte, error) { return transform(b, unprotect) }
