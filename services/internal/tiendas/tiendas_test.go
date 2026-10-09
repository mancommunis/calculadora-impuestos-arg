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

func TestBattleNetReportaBloqueo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := BattleNet(context.Background(), srv.Client(), srv.URL)
	if err == nil {
		t.Fatal("esperaba error por anti-bot")
	}
	if !contains(err.Error(), "401") {
		t.Fatalf("el error debería mencionar 401: %v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
