package motor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buscarWasm ubica el artefacto del motor subiendo desde el cwd del test.
func buscarWasm(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("MOTOR_WASM"); p != "" {
		return p
	}
	rel := filepath.Join("core", "target", "wasm32-wasip1", "release", "calculadora_core.wasm")
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
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
	return ""
}

func cargar(t *testing.T) (*Motor, context.Context) {
	t.Helper()
	wasm := buscarWasm(t)
	if wasm == "" {
		t.Skip("wasm no encontrado; correr 'make core' primero")
	}
	ctx := context.Background()
	m, err := Nuevo(ctx, wasm)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Cerrar(ctx) })
	return m, ctx
}

func TestRoundTrip(t *testing.T) {
	m, ctx := cargar(t)

	entrada := `{
		"precio": { "moneda": "usd", "monto": 10 },
		"regimen": "servicios_digitales",
		"parametros": { "iva": 0.21, "ganancias": 0.30, "iibb": 0.02, "impuesto_pais": 0.0 },
		"medio": { "nombre": "oficial", "tipo": "tarjeta_local", "fx": "oficial", "recargo_extra": 0.0 },
		"cotizaciones": { "oficial": 1000.0 }
	}`
	salida, err := m.CalcularJSON(ctx, entrada)
	if err != nil {
		t.Fatal(err)
	}

	var d struct {
		TotalARS float64 `json:"total_ars"`
	}
	if err := json.Unmarshal([]byte(salida), &d); err != nil {
		t.Fatalf("salida no es JSON: %v (%s)", err, salida)
	}
	if d.TotalARS != 15300 {
		t.Fatalf("total_ars = %v, esperado 15300", d.TotalARS)
	}
}

func TestErrorDeNegocio(t *testing.T) {
	m, ctx := cargar(t)

	entrada := `{
		"precio": { "moneda": "usd", "monto": 10 },
		"regimen": "gaming",
		"parametros": { "iva": 0.21, "ganancias": 0.0, "iibb": 0.02 },
		"medio": { "nombre": "oficial", "tipo": "tarjeta_local", "fx": "oficial" },
		"cotizaciones": {}
	}`
	salida, err := m.CalcularJSON(ctx, entrada)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(salida, "error") {
		t.Fatalf("esperaba error de negocio, obtuve: %s", salida)
	}
}
