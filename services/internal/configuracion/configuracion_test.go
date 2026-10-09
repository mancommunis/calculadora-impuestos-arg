package configuracion

import "testing"

func TestBuscarPorAlias(t *testing.T) {
	servs := []Servicio{
		{ID: "wow", Nombre: "World of Warcraft", Aliases: []string{"WoW Forever", "el wow"}},
		{ID: "chatgpt_plus", Nombre: "ChatGPT Plus"},
	}
	got := Buscar(servs, "wow forever")
	if len(got) != 1 || got[0].ID != "wow" {
		t.Fatalf("esperaba 'wow', obtuve %+v", got)
	}
	if len(Buscar(servs, "CHAT")) != 1 {
		t.Fatal("esperaba match de chatgpt (case-insensitive, prefijo)")
	}
	if len(Buscar(servs, "zzz")) != 0 {
		t.Fatal("no debería matchear")
	}
	if len(Buscar(servs, "  ")) != 0 {
		t.Fatal("query vacía no devuelve nada")
	}
}

func TestParametros(t *testing.T) {
	c := Config{Fiscal: TaxConfig{
		ImpuestoPais: 0,
		Regimenes: map[string]RegimenParams{
			"gaming":              {IVA: 0.21, Ganancias: 0},
			"servicios_digitales": {IVA: 0.21, Ganancias: 0.30},
		},
		IIBB: map[string]float64{"CABA": 0.02},
	}}
	p, err := c.Parametros("servicios_digitales", "CABA")
	if err != nil {
		t.Fatal(err)
	}
	if p.IVA != 0.21 || p.Ganancias != 0.30 || p.IIBB != 0.02 {
		t.Fatalf("parámetros inesperados: %+v", p)
	}
	if _, err := c.Parametros("gaming", "Marte"); err == nil {
		t.Fatal("esperaba error por provincia sin IIBB")
	}
	if _, err := c.Parametros("nope", "CABA"); err == nil {
		t.Fatal("esperaba error por régimen desconocido")
	}
}

func TestExpandirWallets(t *testing.T) {
	w := Wallets{
		Perfiles: map[string]Perfil{
			"banco_local": {Tipo: "tarjeta_local", Origen: "local", Fx: "oficial", Monedas: []string{"USD", "ARS"}, AceptaEn: []string{"*"}},
			"global":      {Tipo: "tarjeta_global", Origen: "global", Fx: "cripto", Monedas: []string{"USD"}, AceptaEn: []string{"*"}},
		},
		Emisores: []Emisor{
			{ID: "santander", Nombre: "Santander", Perfil: "banco_local"},
			{ID: "astropay", Nombre: "AstroPay", Perfil: "global", SuscripcionUSD: ptr(12.0)},
		},
	}
	medios := w.Expandir()
	if len(medios) != 2 {
		t.Fatalf("esperaba 2 medios, obtuve %d", len(medios))
	}
	if medios[0].ID != "santander" || medios[0].Tipo != "tarjeta_local" || medios[0].Origen != "local" {
		t.Fatalf("santander mal expandido: %+v", medios[0])
	}
	if medios[1].ID != "astropay" || medios[1].Tipo != "tarjeta_global" || medios[1].SuscripcionUSD != 12 {
		t.Fatalf("astropay mal expandido: %+v", medios[1])
	}
}

func ptr(f float64) *float64 { return &f }

func TestMedioAplicaA(t *testing.T) {
	todos := Medio{ID: "oficial", AceptaEn: []string{"*"}, Monedas: []string{"USD", "ARS"}}
	if !todos.AplicaA("battle_net", "USD") {
		t.Fatal("'*' debería aplicar a cualquier tienda")
	}
	soloSteamUSD := Medio{ID: "arq_local", AceptaEn: []string{"steam"}, Monedas: []string{"USD"}}
	if !soloSteamUSD.AplicaA("steam", "USD") {
		t.Fatal("arq_local debería aplicar en steam/USD")
	}
	if soloSteamUSD.AplicaA("battle_net", "USD") {
		t.Fatal("arq_local no debería aplicar en battle_net")
	}
	if soloSteamUSD.AplicaA("steam", "ARS") {
		t.Fatal("arq_local es solo USD")
	}
}
