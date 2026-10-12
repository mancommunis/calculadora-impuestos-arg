// Comando enlazar-steam: para cada juego de Battle.net (dataset/battlenet.json)
// busca su appid en Steam por nombre y escribe dataset/steam.json con la MISMA
// canónica, para poder comparar el mismo juego entre tiendas.
package main

import (
	"context"
	"flag"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/configuracion"
	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/tiendas"
)

type ref struct {
	nombre   string
	grupo    string
	subgrupo string
}

func main() {
	bn := flag.String("battlenet", "dataset/battlenet.json", "catálogo de Battle.net")
	out := flag.String("out", "dataset/steam.json", "salida")
	cc := flag.String("cc", "ar", "país de Steam")
	flag.Parse()
	if err := correr(*bn, *out, *cc); err != nil {
		log.Fatalf("enlazar-steam: %v", err)
	}
}

func correr(bnPath, out, cc string) error {
	cat, err := configuracion.CargarCatalogo(bnPath)
	if err != nil {
		return err
	}
	// canónicas de juegos/ediciones
	canonicas := map[string]ref{}
	for _, p := range cat.Productos {
		if p.Canonica == "" {
			continue
		}
		if p.Subgrupo != "Juegos y expansiones" && p.Subgrupo != "Ediciones" {
			continue
		}
		if _, ok := canonicas[p.Canonica]; !ok {
			canonicas[p.Canonica] = ref{nombre: strings.ReplaceAll(p.Canonica, "-", " "), grupo: p.Grupo, subgrupo: p.Subgrupo}
		}
	}

	ctx := context.Background()
	cliente := tiendas.Cliente()
	var steam []configuracion.Servicio
	vistos := map[int]bool{}
	for canonica, rf := range canonicas {
		items, err := tiendas.SteamBuscar(ctx, cliente, rf.nombre, cc)
		if err != nil {
			continue
		}
		app := elegirApp(items, rf.nombre)
		if app == nil || vistos[app.AppID] {
			continue
		}
		vistos[app.AppID] = true
		var precio *float64
		moneda := "USD"
		if pt, err := tiendas.Steam(ctx, cliente, "", app.AppID, cc); err == nil {
			m := pt.Monto
			precio = &m
			moneda = tiendas.NormalizaMoneda(pt.Moneda)
		}
		steam = append(steam, configuracion.Servicio{
			ID: "steam:" + strconv.Itoa(app.AppID), Nombre: limpiar(app.Name), Canonica: canonica,
			Grupo: rf.grupo, Subgrupo: rf.subgrupo,
			Regimen: "gaming", Moneda: moneda, Tienda: "steam", AppID: app.AppID, Precio: precio,
		})
		log.Printf("  %s -> steam:%d %s (%v)", canonica, app.AppID, limpiar(app.Name), precio)
		time.Sleep(120 * time.Millisecond)
	}

	sort.Slice(steam, func(i, j int) bool { return steam[i].ID < steam[j].ID })
	if err := configuracion.GuardarCatalogo(out, configuracion.Catalogo{Productos: steam}); err != nil {
		return err
	}
	log.Printf("enlazados %d juegos con Steam -> %s", len(steam), out)
	return nil
}

var reMarcas = regexp.MustCompile(`[®™©]`)

func limpiar(s string) string {
	return strings.Join(strings.Fields(reMarcas.ReplaceAllString(s, "")), " ")
}

func norm(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(reMarcas.ReplaceAllString(s, " "))), " ")
}

func tokens(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r == ' ' || r == '-' || r == ':' || r == '—'
	})
}

// ruido: resultados que NO son el juego base (soundtrack, arte, upgrades...).
var ruido = []string{"soundtrack", "banda sonora", "artbook", "art book", "original", "upgrade", "cosmetic", "pack", "bundle", "temporada", "season", "battle pass", "dlc", "expansion", "add-on", "addon"}

func elegirApp(items []tiendas.AppSteam, query string) *tiendas.AppSteam {
	q := tokens(query)
	for i := range items {
		n := norm(items[i].Name)
		esRuido := false
		for _, r := range ruido {
			if strings.Contains(n, r) {
				esRuido = true
				break
			}
		}
		if esRuido {
			continue
		}
		ok := len(q) > 0
		for _, t := range q {
			if !strings.Contains(n, t) {
				ok = false
				break
			}
		}
		// Con un solo token, exigí match exacto (evita "diablo" -> "Diablo IV").
		if ok && len(q) == 1 && n != norm(query) {
			ok = false
		}
		if ok {
			return &items[i]
		}
	}
	return nil
}
