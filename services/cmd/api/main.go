// Comando api: servidor HTTP mínimo sobre el motor fiscal.
//
//	POST /calcular   — recibe JSON de Entrada, devuelve JSON de Desglose
//	GET  /health     — "ok"
package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/motor"
)

func main() {
	rutaWasm := flag.String("wasm", rutaWasmPorDefecto(), "ruta al wasm del motor")
	addr := flag.String("addr", ":8080", "dirección de escucha")
	flag.Parse()

	ctx := context.Background()
	m, err := motor.Nuevo(ctx, *rutaWasm)
	if err != nil {
		log.Fatalf("no se pudo cargar el motor: %v", err)
	}
	defer func() { _ = m.Cerrar(ctx) }()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	mux.HandleFunc("/calcular", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "usar POST", http.StatusMethodNotAllowed)
			return
		}
		cuerpo, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			http.Error(w, "no se pudo leer el cuerpo", http.StatusBadRequest)
			return
		}
		salida, err := m.CalcularJSON(r.Context(), string(cuerpo))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, salida)
	})

	log.Printf("api escuchando en %s (motor: %s)", *addr, *rutaWasm)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func rutaWasmPorDefecto() string {
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
