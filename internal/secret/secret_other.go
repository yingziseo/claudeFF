//go:build !windows

package secret

import "errors"

type Vault struct{}

func (Vault) Protect([]byte) ([]byte, error) {
	return nil, errors.New("客户端秘密存储需要 Windows DPAPI")
}
func (Vault) Unprotect([]byte) ([]byte, error) {
	return nil, errors.New("客户端秘密存储需要 Windows DPAPI")
}
