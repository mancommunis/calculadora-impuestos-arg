// Package llaves guarda las API keys de proveedores en el keychain del sistema
// operativo (macOS Keychain, Linux Secret Service, Windows Credential Vault).
// Nunca van a disco plano ni a git.
package llaves

import "github.com/zalando/go-keyring"

// servicio agrupa las entradas en el keychain.
const servicio = "mancommunis"

// Get devuelve la llave de un proveedor (ej. "deepseek").
func Get(proveedor string) (string, error) {
	return keyring.Get(servicio, proveedor)
}

// Set guarda (o reemplaza) la llave de un proveedor.
func Set(proveedor, valor string) error {
	return keyring.Set(servicio, proveedor, valor)
}

// Del borra la llave de un proveedor.
func Del(proveedor string) error {
	return keyring.Delete(servicio, proveedor)
}
