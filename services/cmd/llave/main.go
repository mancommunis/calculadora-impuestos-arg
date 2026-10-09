// Comando llave: administra las API keys de proveedores en el keychain del SO.
//
//	llave set <proveedor> [valor]   (sin valor, lo lee por stdin)
//	llave get <proveedor>
//	llave del <proveedor>
//
// Las llaves nunca se guardan en disco plano ni en git.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/llaves"
)

func main() {
	if len(os.Args) < 2 {
		uso()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "set":
		if len(os.Args) < 3 {
			uso()
			os.Exit(1)
		}
		prov := os.Args[2]
		var valor string
		if len(os.Args) >= 4 {
			valor = os.Args[3]
		} else {
			b, _ := io.ReadAll(io.LimitReader(os.Stdin, 1<<16))
			valor = strings.TrimSpace(string(b))
		}
		if valor == "" {
			fatal("valor vacío")
		}
		if err := llaves.Set(prov, valor); err != nil {
			fatal(err.Error())
		}
		fmt.Printf("guardada la llave de %q en el keychain\n", prov)
	case "get":
		if len(os.Args) < 3 {
			uso()
			os.Exit(1)
		}
		k, err := llaves.Get(os.Args[2])
		if err != nil {
			fatal(err.Error())
		}
		fmt.Println(k)
	case "del":
		if len(os.Args) < 3 {
			uso()
			os.Exit(1)
		}
		if err := llaves.Del(os.Args[2]); err != nil {
			fatal(err.Error())
		}
		fmt.Printf("borrada la llave de %q\n", os.Args[2])
	default:
		uso()
		os.Exit(1)
	}
}

func uso() {
	fmt.Fprintln(os.Stderr, "uso: llave <set|get|del> <proveedor> [valor]")
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}
