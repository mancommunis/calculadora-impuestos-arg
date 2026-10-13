package main

import (
	"testing"

	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/configuracion"
)

func precio(f float64) *float64 { return &f }

// El mismo producto figura en dos páginas de battle.net (la propia y una oferta
// cruzada): el dataset no debe listarlo dos veces.
func TestDedupeMismoProductoEnDosPaginas(t *testing.T) {
	productos := []configuracion.Servicio{
		{
			ID: "battlenet:warcraft-3-reforged:warcraft-forever-collection", Nombre: "Warcraft Forever Collection",
			Grupo: "Warcraft", Subgrupo: "Forever", Canonica: "warcraft-3-reforged",
			Tienda: "battle_net", Moneda: "USD", Precio: precio(79.99),
		},
		{
			ID: "battlenet:world-of-warcraft-forever:warcraft-forever-collection", Nombre: "Warcraft Forever Collection",
			Grupo: "World of Warcraft", Subgrupo: "Forever", Canonica: "world-of-warcraft-forever",
			Tienda: "battle_net", Moneda: "USD", Precio: precio(79.99),
		},
	}

	got := dedupeProductos(productos)
	if len(got) != 1 {
		t.Fatalf("esperaba 1 entrada, hay %d", len(got))
	}
	// Gana la canónica que nombra al ítem (su página propia), no la oferta cruzada.
	if got[0].Grupo != "World of Warcraft" {
		t.Errorf("ganó grupo=%q canónica=%q; esperaba la de World of Warcraft", got[0].Grupo, got[0].Canonica)
	}
}

// Variantes con precios distintos y productos distintos no son duplicados.
func TestDedupeNoTocaVariantes(t *testing.T) {
	productos := []configuracion.Servicio{
		{ID: "a", Nombre: "Hearthstone Battlegrounds Season Pass", Tienda: "battle_net", Moneda: "USD", Precio: precio(14.99)},
		{ID: "b", Nombre: "Hearthstone Battlegrounds Season Pass", Tienda: "battle_net", Moneda: "USD", Precio: precio(19.99)},
		{ID: "c", Nombre: "Otro producto", Tienda: "battle_net", Moneda: "USD", Precio: precio(14.99)},
		{ID: "d", Nombre: "Otro producto", Tienda: "steam", Moneda: "USD", Precio: precio(14.99)},
	}
	if got := dedupeProductos(productos); len(got) != 4 {
		t.Fatalf("esperaba 4 entradas, hay %d", len(got))
	}
}
