// Comando descubrir-battlenet: enumera TODOS los productos de Battle.net (via
// sitemap) y los escribe al dataset con sus precios, leyendo el structured data
// (ld+json) que el sitio sirve a los crawlers. No necesita login.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/configuracion"
	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/tiendas"
)

const sitemapIndex = "https://us.shop.battle.net/sitemap_index.xml"

func main() {
	out := flag.String("out", "dataset/battlenet.json", "archivo de salida")
	categorias := flag.String("categorias", "config/categorias.json", "overrides de categoría")
	delay := flag.Duration("delay", 120*time.Millisecond, "pausa entre requests")
	flag.Parse()
	if err := correr(*out, cargarOverrides(*categorias), *delay); err != nil {
		log.Fatalf("descubrir-battlenet: %v", err)
	}
}

func correr(out string, ov map[string]string, delay time.Duration) error {
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
		agrega := func(id, nm string, moneda string, precio float64, edicion string) {
			productos = append(productos, configuracion.Servicio{
				ID: id, Nombre: nm, Grupo: franchiseDe(slug, nm),
				Subgrupo: override(ov, id, slug, categoriaDe(slug, nm)),
				Canonica: slug,
				Regimen:  "gaming", Moneda: moneda, Tienda: "battle_net",
				URL: u, Edicion: edicion, Precio: &precio,
			})
		}
		if len(ofertas) == 1 {
			nm := limpiarNombre(nombre)
			if nm == "" {
				nm = limpiarNombre(ofertas[0].Nombre)
			}
			agrega("battlenet:"+slug, nm, ofertas[0].Moneda, ofertas[0].Precio, "")
		} else {
			for _, o := range ofertas {
				nn := limpiarNombre(o.Nombre)
				agrega("battlenet:"+slug+":"+slugDe(o.Nombre), nn, o.Moneda, o.Precio, nn)
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

	// El mismo producto aparece en más de una página (la propia y como oferta
	// cruzada en otras): se conserva una sola entrada.
	productos = dedupeProductos(productos)

	sort.Slice(productos, func(i, j int) bool { return productos[i].ID < productos[j].ID })
	if err := configuracion.GuardarCatalogo(out, configuracion.Catalogo{Productos: productos}); err != nil {
		return err
	}
	log.Printf("escrito %s: %d entradas (%d con precio, %d sin)", out, len(productos), conPrecio, sinPrecio)
	fmt.Printf("ok: %d entradas\n", len(productos))
	return nil
}

// ---- slug / franquicia ----

var reSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugDe(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return strings.Trim(reSlug.ReplaceAllString(s, "-"), "-")
}

// dedupeProductos descarta entradas repetidas del mismo producto: battle.net lo
// publica en varias páginas (la propia y como oferta cruzada en otras). La
// identidad es nombre+tienda+moneda+precio — NO el grupo, que es justo lo que
// cambia según la página donde aparezca. De las repetidas gana la mejor
// clasificada: la canónica que comparte más palabras con el nombre del ítem.
func dedupeProductos(productos []configuracion.Servicio) []configuracion.Servicio {
	enSalida := map[string]int{}
	salida := make([]configuracion.Servicio, 0, len(productos))
	for _, p := range productos {
		k := claveProducto(p)
		i, visto := enSalida[k]
		if !visto {
			enSalida[k] = len(salida)
			salida = append(salida, p)
			continue
		}
		if pesoCanonica(p.Nombre, p.Canonica) > pesoCanonica(salida[i].Nombre, salida[i].Canonica) {
			salida[i] = p
		}
	}
	return salida
}

// claveProducto identifica al producto más allá de en qué página se lo encontró.
func claveProducto(p configuracion.Servicio) string {
	var precio float64
	if p.Precio != nil {
		precio = *p.Precio
	}
	return strings.ToLower(strings.TrimSpace(p.Nombre)) + "|" +
		p.Tienda + "|" + p.Moneda + "|" + strconv.FormatFloat(precio, 'f', 2, 64)
}

// pesoCanonica cuenta cuántas palabras comparte la canónica (slug de la página)
// con el nombre del ítem: la página propia lo nombra en su slug, la de una
// oferta cruzada no. Desempata entre entradas repetidas.
func pesoCanonica(nombre, canonica string) int {
	palabras := map[string]bool{}
	for _, tok := range strings.Split(slugDe(nombre), "-") {
		palabras[tok] = true
	}
	n := 0
	for _, tok := range strings.Split(canonica, "-") {
		if palabras[tok] {
			n++
		}
	}
	return n
}

var franquiciasSlug = []struct{ prefijo, nombre string }{
	{"world-of-warcraft", "World of Warcraft"},
	{"wow-", "World of Warcraft"},
	{"diablo-iv", "Diablo IV"},
	{"diablo-iii", "Diablo III"},
	{"diablo-ii", "Diablo II"},
	{"eternal-orbs", "Diablo Immortal"},
	{"diablo", "Diablo"},
	{"call-of-duty", "Call of Duty"},
	{"hearthstone", "Hearthstone"},
	{"starcraft", "StarCraft"},
	{"heroes-of-the-storm", "Heroes of the Storm"},
	{"overwatch", "Overwatch"},
	{"warcraft-rumble", "Warcraft Rumble"},
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

// Por nombre de oferta (para slugs que no arrancan con la franquicia: announcers, commanders...).
var franquiciasNombre = []struct{ kw, nombre string }{
	{"starcraft", "StarCraft"},
	{"world of warcraft", "World of Warcraft"},
	{"warcraft", "Warcraft"},
	{"diablo", "Diablo"},
	{"overwatch", "Overwatch"},
	{"hearthstone", "Hearthstone"},
	{"call of duty", "Call of Duty"},
	{"heroes of the storm", "Heroes of the Storm"},
	{"sea of thieves", "Sea of Thieves"},
	{"doom", "DOOM"},
	{"avowed", "Avowed"},
	{"the outer worlds", "The Outer Worlds"},
	{"tony hawk", "Tony Hawk's Pro Skater"},
	{"crash bandicoot", "Crash Bandicoot"},
	{"blizzard arcade", "Blizzard Arcade"},
}

func franchiseDe(slug, nombre string) string {
	for _, f := range franquiciasSlug {
		if strings.HasPrefix(slug, f.prefijo) {
			return f.nombre
		}
	}
	n := strings.ToLower(nombre)
	for _, f := range franquiciasNombre {
		if strings.Contains(n, f.kw) {
			return f.nombre
		}
	}
	return ""
}

var reMarcas = regexp.MustCompile(`[®™©]`)

// limpiarNombre saca ®/™/© y espacios dobles (ej. "Warcraft® III").
func limpiarNombre(s string) string {
	s = reMarcas.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), " ")
}

// ---- categoría ----

var (
	reMonedas    = regexp.MustCompile(`\b(platinum|gold|tokens?|coins?|hearthsteel)\b`)
	rePases      = regexp.MustCompile(`\b(season pass|battle pass|pass)\b`)
	reServicios  = regexp.MustCompile(`\b(services?|boosts?|transfers?|faction|race change|character|name change)\b`)
	reCosmeticos = regexp.MustCompile(`\b(transmogs?|announcers?|ensembles?|raiment|crown|cosmetics?|appearances?|skins?)\b`)
	reColeccion  = regexp.MustCompile(`\b(collections?)\b`)
	rePacks      = regexp.MustCompile(`\b(packs?|bundles?|mini[- ]?sets?|class sets?)\b`)
	reEdiciones  = regexp.MustCompile(`\b(editions?|deluxe|ultimate|premium)\b`)
)

// categoriaDe infiere el subgrupo (categoría) del NOMBRE de la oferta (con el slug de respaldo).
func categoriaDe(slug, nombre string) string {
	s := strings.ToLower(slug)
	n := strings.ToLower(nombre)
	switch {
	case strings.Contains(n, "forever"):
		return "Forever"
	case rePases.MatchString(n) || rePases.MatchString(s):
		return "Pases"
	case strings.Contains(n, "game time") || strings.Contains(s, "game-time") || strings.Contains(s, "subscription"):
		return "Suscripción"
	case strings.Contains(s, "-mount-"):
		return "Monturas"
	case strings.Contains(s, "-pet-"):
		return "Mascotas"
	case reMonedas.MatchString(n) || reMonedas.MatchString(s):
		return "Monedas"
	case reServicios.MatchString(s):
		return "Servicios"
	case reCosmeticos.MatchString(n) || reCosmeticos.MatchString(s) || reColeccion.MatchString(n) || reColeccion.MatchString(s):
		return "Cosméticos y colecciones"
	case rePacks.MatchString(n) || rePacks.MatchString(s):
		return "Packs y bundles"
	case reEdiciones.MatchString(n) || reEdiciones.MatchString(s):
		return "Ediciones"
	default:
		return "Juegos y expansiones"
	}
}

// ---- overrides de categoría (config/categorias.json) ----

func cargarOverrides(path string) map[string]string {
	ov := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return ov
	}
	var f struct {
		Overrides map[string]string `json:"overrides"`
	}
	if json.Unmarshal(b, &f) != nil {
		return ov
	}
	return f.Overrides
}

func override(ov map[string]string, id, slug, cat string) string {
	if c, ok := ov[id]; ok {
		return c
	}
	if c, ok := ov[slug]; ok {
		return c
	}
	return cat
}
