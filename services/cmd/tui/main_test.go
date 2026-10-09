package main

import (
	"context"
	"testing"

	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/configuracion"
	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/motor"
)

// traerFx debe parsear el JSON de dolarapi, que mezcla números y strings
// (nombre, casa, fechaActualizacion). Regresión del bug "todo en 0".
func TestTraerFx(t *testing.T) {
	fx, err := traerFx(context.Background())
	if err != nil {
		t.Skipf("sin red/apis: %v", err)
	}
	for _, k := range []string{"oficial", "mep", "cripto"} {
		if fx[k] <= 0 {
			t.Fatalf("fx[%q] debería ser > 0; fx=%v", k, fx)
		}
	}
}

// End-to-end: carga config + motor y calcula DeepSeek; ningún total puede dar 0.
func TestDeepSeekCalcula(t *testing.T) {
	ctx := context.Background()
	m, err := motor.Nuevo(ctx, motor.RutaWasmPorDefecto())
	if err != nil {
		t.Skipf("sin wasm (correr 'make core'): %v", err)
	}
	cfg, err := configuracion.Cargar(configuracion.DirPorDefecto())
	if err != nil {
		t.Fatalf("cargando config: %v", err)
	}
	fx, err := traerFx(ctx)
	if err != nil || len(fx) == 0 {
		t.Skipf("sin cotizaciones: %v", err)
	}

	var ds *configuracion.Servicio
	for i := range cfg.Servicios {
		if cfg.Servicios[i].ID == "deepseek_recarga" {
			ds = &cfg.Servicios[i]
		}
	}
	if ds == nil {
		t.Fatal("no está 'deepseek_recarga' en la config")
	}

	mod := inicial()
	mod.motor = m
	mod.cfg = cfg
	mod.fx = fx
	mod.provincia = "Cordoba"
	mod.seleccion = ds
	mod.monto = *ds.Precio
	mod.recalcular()

	if len(mod.calculos) == 0 {
		t.Fatal("no se calculó ningún medio para DeepSeek")
	}
	for _, c := range mod.calculos {
		if c.desglose.TotalARS <= 0 {
			t.Fatalf("total en 0 para %s: %+v", c.medio.Nombre, c.desglose)
		}
	}
	t.Logf("%d medios · mejor: %s = $%.2f", len(mod.calculos), mod.calculos[0].medio.Nombre, mod.calculos[0].desglose.TotalARS)
}
