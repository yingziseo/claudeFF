//go:build darwin

package secret

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <Security/Security.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>
#include <string.h>
static int vault_key(unsigned char *out, unsigned char *generated) {
 CFStringRef service=CFSTR("com.claudeff.workspace.proxy-key");
 const void *keys[]={kSecClass,kSecAttrService,kSecAttrAccount,kSecReturnData,kSecMatchLimit};
 const void *values[]={kSecClassGenericPassword,service,CFSTR("local-user"),kCFBooleanTrue,kSecMatchLimitOne};
 CFDictionaryRef query=CFDictionaryCreate(NULL,keys,values,5,&kCFTypeDictionaryKeyCallBacks,&kCFTypeDictionaryValueCallBacks);
 CFTypeRef result=NULL;OSStatus status=SecItemCopyMatching(query,&result);CFRelease(query);
 if(status==errSecItemNotFound){
  CFDataRef data=CFDataCreate(NULL,generated,32);
  const void *ak[]={kSecClass,kSecAttrService,kSecAttrAccount,kSecValueData};
  const void *av[]={kSecClassGenericPassword,service,CFSTR("local-user"),data};
  CFDictionaryRef add=CFDictionaryCreate(NULL,ak,av,4,&kCFTypeDictionaryKeyCallBacks,&kCFTypeDictionaryValueCallBacks);
  status=SecItemAdd(add,NULL);CFRelease(add);CFRelease(data);
  if(status==errSecSuccess){memcpy(out,generated,32);return 0;}
  return (int)status;
 }
 if(status!=errSecSuccess){return (int)status;}
 if(CFGetTypeID(result)!=CFDataGetTypeID()||CFDataGetLength((CFDataRef)result)!=32){CFRelease(result);return -1;}
 memcpy(out,CFDataGetBytePtr((CFDataRef)result),32);CFRelease(result);return 0;
}
*/
import "C"
import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"sync"
	"unsafe"
)

type Vault struct{}

var keyMu sync.Mutex

func key() ([]byte, error) {
	keyMu.Lock()
	defer keyMu.Unlock()
	b := make([]byte, 32)
	g := make([]byte, 32)
	if _, e := rand.Read(g); e != nil {
		return nil, e
	}
	if C.vault_key((*C.uchar)(unsafe.Pointer(&b[0])), (*C.uchar)(unsafe.Pointer(&g[0]))) != 0 {
		return nil, errors.New("macOS 钥匙串访问失败，请允许当前程序访问")
	}
	return b, nil
}
func aead() (cipher.AEAD, error) {
	k, e := key()
	if e != nil {
		return nil, e
	}
	b, e := aes.NewCipher(k)
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(b)
}
func (Vault) Protect(b []byte) ([]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}
	a, e := aead()
	if e != nil {
		return nil, e
	}
	n := make([]byte, a.NonceSize())
	if _, e = rand.Read(n); e != nil {
		return nil, e
	}
	return a.Seal(n, n, b, nil), nil
}
func (Vault) Unprotect(b []byte) ([]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}
	a, e := aead()
	if e != nil {
		return nil, e
	}
	if len(b) < a.NonceSize() {
		return nil, errors.New("钥匙串秘密数据无效")
	}
	return a.Open(nil, b[:a.NonceSize()], b[a.NonceSize():], nil)
}
