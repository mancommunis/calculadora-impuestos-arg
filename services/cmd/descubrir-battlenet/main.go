// Comando descubrir-battlenet: enumera TODOS los productos de Battle.net (via
// sitemap) y los escribe al dataset con sus precios, leyendo el structured data
// (ld+json) que el sitio sirve a los crawlers. No necesita login.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/configuracion"
	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/tiendas"
)

const sitemapIndex = "https://us.shop.battle.net/sitemap_index.xml"

func main() {
	out := flag.String("out", "dataset/battlenet.json", "archivo de salida")
	delay := flag.Duration("delay", 120*time.Millisecond, "pausa entre requests")
	flag.Parse()
	if err := correr(*out, *delay); err != nil {
		log.Fatalf("descubrir-battlenet: %v", err)
	}
}

func correr(out string, delay time.Duration) error {
	ctx := context.Background()
	cliente := tiendas.Cliente()

	urls, err := tiendas.BattleNetSitemapProductos(ctx, cliente, sitemapIndex)
	if err != nil {
		return err
	}
	log.Printf("productos en el sitemap: %d", len(urls))

	var productos []configuracion.Servicio
	var conPrecio, sinPrecio int
	for i, u := range urls {
		nombre, ofertas, err := tiendas.BattleNetProducto(ctx, cliente, u)
		if err != nil {
			sinPrecio++
			continue
		}
		slug := slugDe(u)
		grupo := franchiseDe(slug)
		if len(ofertas) == 1 {
			nm := nombre
			if nm == "" {
				nm = ofertas[0].Nombre
			}
			precio := ofertas[0].Precio
			productos = append(productos, configuracion.Servicio{
				ID: "battlenet:" + slug, Nombre: nm, Grupo: grupo,
				Regimen: "gaming", Moneda: ofertas[0].Moneda, Tienda: "battle_net",
				URL: u, Precio: &precio,
			})
		} else {
			for _, o := range ofertas {
				precio := o.Precio
				productos = append(productos, configuracion.Servicio{
					ID: "battlenet:" + slug + ":" + slugDe(o.Nombre), Nombre: o.Nombre, Grupo: grupo,
					Regimen: "gaming", Moneda: o.Moneda, Tienda: "battle_net",
					URL: u, Edicion: o.Nombre, Precio: &precio,
				})
			}
		}
		conPrecio++
		if delay > 0 && i < len(urls)-1 {
			time.Sleep(delay)
		}
		if (i+1)%50 == 0 {
			log.Printf("  %d/%d…", i+1, len(urls))
		}
	}

	sort.Slice(productos, func(i, j int) bool { return productos[i].ID < productos[j].ID })
	if err := configuracion.GuardarCatalogo(out, configuracion.Catalogo{Productos: productos}); err != nil {
		return err
	}
	log.Printf("escrito %s: %d entradas (%d productos con precio, %d sin)", out, len(productos), conPrecio, sinPrecio)
	fmt.Printf("ok: %d entradas\n", len(productos))
	return nil
}

var reSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugDe(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return strings.Trim(reSlug.ReplaceAllString(s, "-"), "-")
}

var franquicias = []struct{ prefijo, nombre string }{
	{"world-of-warcraft", "World of Warcraft"},
	{"wow-", "World of Warcraft"},
	{"diablo-iv", "Diablo IV"},
	{"diablo-iii", "Diablo III"},
	{"diablo-ii", "Diablo II"},
	{"diablo-", "Diablo"},
	{"call-of-duty", "Call of Duty"},
	{"hearthstone", "Hearthstone"},
	{"starcraft", "StarCraft"},
	{"heroes-of-the-storm", "Heroes of the Storm"},
	{"overwatch", "Overwatch"},
	{"warcraft", "Warcraft"},
	{"blizzard-arcade", "Blizzard Arcade"},
	{"crash-bandicoot", "Crash Bandicoot"},
	{"tony-hawks", "Tony Hawk's Pro Skater"},
	{"the-outer-worlds", "The Outer Worlds"},
	{"avowed", "Avowed"},
	{"sea-of-thieves", "Sea of Thieves"},
	{"doom", "DOOM"},
	{"hexen", "Heretic/Hexen"},
	{"bulletstorm", "Bulletstorm"},
}

func franchiseDe(slug string) string {
	for _, f := range franquicias {
		if strings.HasPrefix(slug, f.prefijo) {
			return f.nombre
		}
	}
	return ""
}
