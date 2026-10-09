package motor

import (
	"os"
	"path/filepath"
)

// RutaWasmPorDefecto busca el artefacto wasip1 subiendo desde el cwd, o usa MOTOR_WASM.
func RutaWasmPorDefecto() string {
	if p := os.Getenv("MOTOR_WASM"); p != "" {
		return p
	}
	rel := filepath.Join("core", "target", "wasm32-wasip1", "release", "calculadora_core.wasm")
	dir, err := os.Getwd()
	if err != nil {
		return rel
	}
	for i := 0; i < 8; i++ {
		cand := filepath.Join(dir, rel)
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			break
		}
		dir = padre
	}
	return rel
}
