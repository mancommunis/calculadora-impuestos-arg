// Package motor carga el motor fiscal (Rust → WASM) y lo ejecuta desde Go con
// wazero, sin CGO. El motor no hace I/O: recibe JSON y devuelve JSON.
package motor

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Motor es una instancia cargada del motor fiscal.
type Motor struct {
	runtime  wazero.Runtime
	module   api.Module
	alloc    api.Function
	free     api.Function
	calcular api.Function
}

// Nuevo carga el módulo WASM desde la ruta dada.
func Nuevo(ctx context.Context, rutaWasm string) (*Motor, error) {
	wasm, err := os.ReadFile(rutaWasm)
	if err != nil {
		return nil, fmt.Errorf("leyendo wasm %q: %w", rutaWasm, err)
	}

	r := wazero.NewRuntime(ctx)
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		_ = r.Close(ctx)
		return nil, fmt.Errorf("instanciando wasi: %w", err)
	}

	compilado, err := r.CompileModule(ctx, wasm)
	if err != nil {
		_ = r.Close(ctx)
		return nil, fmt.Errorf("compilando wasm: %w", err)
	}

	// No correr _start: el módulo es una librería, no un comando.
	cfg := wazero.NewModuleConfig().WithName("motor").WithStartFunctions()
	mod, err := r.InstantiateModule(ctx, compilado, cfg)
	if err != nil {
		_ = r.Close(ctx)
		return nil, fmt.Errorf("instanciando módulo: %w", err)
	}

	m := &Motor{
		runtime:  r,
		module:   mod,
		alloc:    mod.ExportedFunction("cfg_alloc"),
		free:     mod.ExportedFunction("cfg_free"),
		calcular: mod.ExportedFunction("cfg_calcular_json"),
	}
	if m.alloc == nil || m.free == nil || m.calcular == nil {
		_ = r.Close(ctx)
		return nil, fmt.Errorf("el módulo no exporta el ABI esperado (cfg_alloc/cfg_free/cfg_calcular_json)")
	}
	return m, nil
}

// Cerrar libera el runtime.
func (m *Motor) Cerrar(ctx context.Context) error {
	return m.runtime.Close(ctx)
}

// CalcularJSON recibe el JSON de entrada y devuelve el JSON de salida.
// Los problemas de negocio (ej.: falta cotización) vuelven como `{"error": ...}`,
// no como error de Go.
func (m *Motor) CalcularJSON(ctx context.Context, entrada string) (string, error) {
	in := []byte(entrada)
	mem := m.module.Memory()

	res, err := m.alloc.Call(ctx, uint64(len(in)))
	if err != nil {
		return "", fmt.Errorf("cfg_alloc: %w", err)
	}
	ptrIn := uint32(res[0])
	if ptrIn == 0 && len(in) > 0 {
		return "", fmt.Errorf("cfg_alloc devolvió null")
	}
	if len(in) > 0 && !mem.Write(ptrIn, in) {
		return "", fmt.Errorf("no se pudo escribir la entrada en memoria")
	}

	res, err = m.calcular.Call(ctx, uint64(ptrIn), uint64(len(in)))
	if err != nil {
		return "", fmt.Errorf("cfg_calcular_json: %w", err)
	}
	ptrOut := uint32(res[0])
	if ptrOut == 0 {
		return "", fmt.Errorf("cfg_calcular_json devolvió null")
	}

	hdr, ok := mem.Read(ptrOut, 4)
	if !ok {
		return "", fmt.Errorf("no se pudo leer el header de salida")
	}
	n := binary.LittleEndian.Uint32(hdr)
	var salida string
	if n > 0 {
		datos, ok := mem.Read(ptrOut+4, n)
		if !ok {
			return "", fmt.Errorf("no se pudo leer la salida")
		}
		salida = string(datos)
	}

	// Liberar los buffers.
	if len(in) > 0 {
		_, _ = m.free.Call(ctx, uint64(ptrIn), uint64(len(in)))
	}
	_, _ = m.free.Call(ctx, uint64(ptrOut), uint64(4+n))

	return salida, nil
}
