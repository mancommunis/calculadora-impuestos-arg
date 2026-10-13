package main

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/configuracion"
	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/motor"
)

func TestTeclasMonto(t *testing.T) {
	suman := []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'+'}},
		{Type: tea.KeyRunes, Runes: []rune{'='}},
		{Type: tea.KeyRight},
	}
	for _, k := range suman {
		if !teclaSuma(k) {
			t.Fatalf("%q debería sumar", k.String())
		}
	}
	sumanMal := []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'-'}},
		{Type: tea.KeyLeft},
	}
	for _, k := range sumanMal {
		if teclaSuma(k) {
			t.Fatalf("%q NO debería sumar", k.String())
		}
	}
	if !teclaResta(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}}) {
		t.Fatal("'-' debería restar")
	}
	if !teclaResta(tea.KeyMsg{Type: tea.KeyLeft}) {
		t.Fatal("← debería restar")
	}
}

func TestMapearProvincia(t *testing.T) {
	provs := []string{"CABA", "PBA", "Cordoba", "Santa Fe"}
	casos := map[string]string{
		"Cordoba Province":                "Cordoba",
		"Buenos Aires":                    "PBA",
		"Ciudad Autónoma de Buenos Aires": "CABA",
		"Santa Fe Province":               "Santa Fe",
		"Marte":                           "",
	}
	for region, esperado := range casos {
		if got := mapearProvincia(region, provs); got != esperado {
			t.Fatalf("%q -> %q, esperaba %q", region, got, esperado)
		}
	}
}

// parsearCampo debe manejar JSON mixto (números + strings). Regresión del bug "todo en 0".
func TestParsearCampo(t *testing.T) {
	dolarapi := []byte(`{"moneda":"USD","casa":"oficial","nombre":"Oficial","compra":1485,"venta":1535,"fechaActualizacion":"2026-10-09T21:00:00Z"}`)
	if v, err := parsearCampo(dolarapi, "venta"); err != nil || v != 1535 {
		t.Fatalf("venta=%.2f err=%v", v, err)
	}
	if _, err := parsearCampo(dolarapi, "nope"); err == nil {
		t.Fatal("campo ausente debería fallar")
	}
	criptoya := []byte(`{"ask":1624.05,"totalAsk":1624.05,"time":1791585528}`)
	if v, err := parsearCampo(criptoya, "ask"); err != nil || v != 1624.05 {
		t.Fatalf("ask=%.2f err=%v", v, err)
	}
}

func TestTraerFx(t *testing.T) {
	fx, err := traerFx(context.Background())
	if err != nil || len(fx) == 0 {
		t.Skipf("sin red/apis: %v", err)
	}
	if _, ok := fx["oficial"]; !ok {
		t.Skip("dolarapi no respondió (rate limit); el parseo está cubierto por TestParsearCampo")
	}
	if fx["oficial"] <= 0 {
		t.Fatalf("oficial debería ser > 0; fx=%v", fx)
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

// El árbol lista TODOS los ítems con su nombre real (el levantado de la web):
// sin filas paraguas con nombres inventados ("Battle.net → Suscripción") y sin
// saltear el nivel del tipo cuando tiene un solo ítem.
func TestArbolListaTodosLosItemsConNombreReal(t *testing.T) {
	precio := func(f float64) *float64 { return &f }
	cfg := configuracion.Config{Servicios: []configuracion.Servicio{
		{ID: "a", Nombre: "World of Warcraft - suscripcion mensual", Grupo: "World of Warcraft", Subgrupo: "Suscripción", Tienda: "battle_net", Moneda: "USD", Precio: precio(14.99)},
		{ID: "b", Nombre: "60 Days of Game Time - World of Warcraft", Grupo: "World of Warcraft", Subgrupo: "Suscripción", Canonica: "world-of-warcraft-game-time", Tienda: "battle_net", Moneda: "USD", Precio: precio(29.99)},
		{ID: "c", Nombre: "WoW Token - World of Warcraft", Grupo: "World of Warcraft", Subgrupo: "Monedas", Canonica: "world-of-warcraft-token", Tienda: "battle_net", Moneda: "USD", Precio: precio(20)},
	}}

	m := inicial()
	m.cfg = cfg
	m.input.SetValue("warcraft")
	m.expandido["g:World of Warcraft"] = true
	m.expandido["s:World of Warcraft|Suscripción"] = true
	m.expandido["s:World of Warcraft|Monedas"] = true
	m.actualizarFilas()

	tipos := map[string]int{}
	lineas := map[string]string{}
	for i, f := range m.filas {
		switch f.tipo {
		case 1:
			tipos[f.nombre] = f.n
		case 2:
			t.Errorf("fila paraguas (tipo 2) inesperada: %q", f.nombre)
		case 3:
			lineas[f.item.ID] = m.lineaFila(i)
		}
	}

	if len(lineas) != len(cfg.Servicios) {
		t.Fatalf("el árbol mostró %d ítems de %d", len(lineas), len(cfg.Servicios))
	}
	if tipos["Monedas"] != 1 {
		t.Errorf("el tipo 'Monedas' (1 ítem) debería tener su renglón; tipos=%v", tipos)
	}
	if tipos["Suscripción"] != 2 {
		t.Errorf("'Suscripción' debería contar 2 ítems; tipos=%v", tipos)
	}

	// Formato del renglón: "Grupo - nombre real del ítem (tienda)" + precio.
	esperados := map[string]string{
		"a": "World of Warcraft - suscripcion mensual (Battle.net)",
		"b": "World of Warcraft - 60 Days of Game Time (Battle.net)",
		"c": "World of Warcraft - WoW Token (Battle.net)",
	}
	for id, esperado := range esperados {
		if !strings.Contains(lineas[id], esperado) {
			t.Errorf("renglón %q no contiene %q", lineas[id], esperado)
		}
	}
	if n := strings.Count(lineas["b"], "World of Warcraft"); n != 1 {
		t.Errorf("el grupo se repite en el renglón (%d veces): %q", n, lineas["b"])
	}
	if !strings.Contains(lineas["b"], "29,99 USD") {
		t.Errorf("falta el precio en el renglón: %q", lineas["b"])
	}
	if strings.Contains(lineas["b"], "→") {
		t.Errorf("el renglón sigue usando un nombre generalizado: %q", lineas["b"])
	}
}
