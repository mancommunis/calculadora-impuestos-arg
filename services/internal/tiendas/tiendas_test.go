package tiendas

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSteamParsePrecio(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"1091500": {
				"success": true,
				"data": {
					"price_overview": { "currency": "USD", "final": 4499, "discount_percent": 10 }
				}
			}
		}`))
	}))
	defer srv.Close()

	got, err := Steam(context.Background(), srv.Client(), srv.URL, 1091500, "ar")
	if err != nil {
		t.Fatal(err)
	}
	if got.Moneda != "USD" || got.Monto != 44.99 || got.Descuento != 10 {
		t.Fatalf("precio inesperado: %+v", got)
	}
	if got.Tienda != "steam" {
		t.Fatalf("tienda inesperada: %s", got.Tienda)
	}
}

func TestSteamSinPrecio(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"1":{"success":true,"data":{"price_overview":null}}}`))
	}))
	defer srv.Close()

	if _, err := Steam(context.Background(), srv.Client(), srv.URL, 1, "ar"); err == nil {
		t.Fatal("esperaba error cuando no hay price_overview")
	}
}

func TestBattleNetParseOfertas(t *testing.T) {
	html := `<html><head>
	<script type="application/ld+json">{"@type":"Product","name":"Midnight","offers":[
	{"@type":"Offer","name":"Midnight Epic Edition","price":"89.99","priceCurrency":"USD"},
	{"@type":"Offer","name":"Midnight Base Edition","price":"49.99","priceCurrency":"USD"}]}</script>
	</head></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	ofertas, err := BattleNet(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(ofertas) != 2 {
		t.Fatalf("esperaba 2 ofertas, obtuve %d", len(ofertas))
	}
	if ofertas[0].Nombre != "Midnight Epic Edition" || ofertas[0].Precio != 89.99 || ofertas[0].Moneda != "USD" {
		t.Fatalf("oferta inesperada: %+v", ofertas[0])
	}
}

func TestBattleNetSinOfertas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>nada</html>"))
	}))
	defer srv.Close()
	if _, err := BattleNet(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Fatal("esperaba error sin structured data")
	}
}
