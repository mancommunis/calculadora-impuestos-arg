// Package tiendas contiene los adaptadores (scrapers/APIs) por tienda.
//
// Primero Steam (API pública, confiable). Battle.net está protegido por
// anti-bot (devuelve 401 incluso con UA de browser): su adaptador lo intenta
// y reporta el bloqueo, sin romper la corrida.
package tiendas

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// PrecioTienda es el precio de un producto en una tienda.
type PrecioTienda struct {
	Tienda    string  `json:"tienda"`
	Moneda    string  `json:"moneda"`
	Monto     float64 `json:"monto"`
	Descuento int     `json:"descuento,omitempty"`
	Fuente    string  `json:"fuente"`
}

// Cliente arma un http.Client con timeout sano.
func Cliente() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

func get(ctx context.Context, c *http.Client, url, ua string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	return c.Do(req)
}

// SteamBase es la base de la API de storefront de Steam.
const SteamBase = "https://store.steampowered.com"

// Steam trae el precio de un appid. `cc` es el país (ej. "ar"); Steam AR cotiza en USD.
func Steam(ctx context.Context, c *http.Client, baseURL string, appid int, cc string) (PrecioTienda, error) {
	if baseURL == "" {
		baseURL = SteamBase
	}
	url := fmt.Sprintf("%s/api/appdetails?appids=%d&cc=%s", baseURL, appid, cc)

	resp, err := get(ctx, c, url, "mancommunis/0.1")
	if err != nil {
		return PrecioTienda{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return PrecioTienda{}, fmt.Errorf("steam status %d", resp.StatusCode)
	}
	cuerpo, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return PrecioTienda{}, err
	}

	var raw map[string]struct {
		Success bool `json:"success"`
		Data    *struct {
			PriceOverview *struct {
				Currency        string `json:"currency"`
				Final           int64  `json:"final"`
				DiscountPercent int    `json:"discount_percent"`
			} `json:"price_overview"`
		} `json:"data"`
	}
	if err := json.Unmarshal(cuerpo, &raw); err != nil {
		return PrecioTienda{}, err
	}
	e, ok := raw[strconv.Itoa(appid)]
	if !ok || !e.Success || e.Data == nil || e.Data.PriceOverview == nil {
		return PrecioTienda{}, fmt.Errorf("steam appid %d sin price_overview", appid)
	}
	po := e.Data.PriceOverview
	return PrecioTienda{
		Tienda:    "steam",
		Moneda:    po.Currency,
		Monto:     float64(po.Final) / 100.0,
		Descuento: po.DiscountPercent,
		Fuente:    url,
	}, nil
}

// Oferta es una edición/variante de un producto en una tienda.
type Oferta struct {
	Nombre string  `json:"nombre"`
	Precio float64 `json:"precio"`
	Moneda string  `json:"moneda"`
}

// battleNetUA: Battle.net deja a los crawlers (SEO) ver el producto sin login.
const battleNetUA = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"

var reLDJSON = regexp.MustCompile(`(?is)<script[^>]*application/ld\+json[^>]*>(.*?)</script>`)
var reSitemap = regexp.MustCompile(`<loc>(https://[^<]*sitemap_en-us\.xml)</loc>`)
var reLoc = regexp.MustCompile(`<loc>([^<]+)</loc>`)

// BattleNet trae las ofertas (ediciones) de una página de producto de Battle.net
// leyendo el structured data (application/ld+json) que el sitio sirve a los crawlers.
// No necesita sesión.
func BattleNet(ctx context.Context, c *http.Client, url string) ([]Oferta, error) {
	_, ofertas, err := BattleNetProducto(ctx, c, url)
	return ofertas, err
}

// BattleNetProducto trae el nombre del producto y sus ofertas.
func BattleNetProducto(ctx context.Context, c *http.Client, url string) (string, []Oferta, error) {
	html, err := leerCrawler(ctx, c, url)
	if err != nil {
		return "", nil, err
	}
	return parsearProducto(html)
}

// BattleNetSitemapProductos enumera las URLs /product/ del sitemap en-us.
func BattleNetSitemapProductos(ctx context.Context, c *http.Client, indexURL string) ([]string, error) {
	idx, err := leerCrawler(ctx, c, indexURL)
	if err != nil {
		return nil, err
	}
	m := reSitemap.FindStringSubmatch(string(idx))
	if m == nil {
		return nil, fmt.Errorf("no encontré sitemap en-us en %s", indexURL)
	}
	sm, err := leerCrawler(ctx, c, m[1])
	if err != nil {
		return nil, err
	}
	var urls []string
	for _, l := range reLoc.FindAllStringSubmatch(string(sm), -1) {
		if strings.Contains(l[1], "/product/") {
			urls = append(urls, l[1])
		}
	}
	return urls, nil
}

func leerCrawler(ctx context.Context, c *http.Client, url string) ([]byte, error) {
	resp, err := get(ctx, c, url, battleNetUA)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d en %s", resp.StatusCode, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

// parsearProducto devuelve el nombre del producto y sus ofertas (del ld+json).
func parsearProducto(html []byte) (string, []Oferta, error) {
	n, ofertas, err := parsear(html)
	if err != nil {
		return "", nil, err
	}
	return n, ofertas, nil
}

func parsear(html []byte) (string, []Oferta, error) {
	var nombre string
	var ofertas []Oferta
	for _, b := range reLDJSON.FindAllStringSubmatch(string(html), -1) {
		if len(b) < 2 {
			continue
		}
		var obj struct {
			Name   string `json:"name"`
			Offers []struct {
				Name          string `json:"name"`
				Price         any    `json:"price"`
				PriceCurrency string `json:"priceCurrency"`
			} `json:"offers"`
		}
		if json.Unmarshal([]byte(b[1]), &obj) != nil {
			continue
		}
		if len(obj.Offers) > 0 {
			if nombre == "" {
				nombre = strings.TrimSuffix(obj.Name, " | Battle.net")
			}
			for _, o := range obj.Offers {
				monto, ok := aNumero(o.Price)
				if !ok {
					continue
				}
				ofertas = append(ofertas, Oferta{Nombre: o.Name, Precio: monto, Moneda: NormalizaMoneda(o.PriceCurrency)})
			}
		}
	}
	if len(ofertas) == 0 {
		return "", nil, fmt.Errorf("sin ofertas en el structured data")
	}
	return nombre, ofertas, nil
}

func parsearOfertas(html []byte) ([]Oferta, error) {
	_, ofertas, err := parsear(html)
	return ofertas, err
}

// aNumero acepta price como string ("89.99") o número.
func aNumero(v any) (float64, bool) {
	switch x := v.(type) {
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	case float64:
		return x, true
	}
	return 0, false
}

// NormalizaMoneda deja la moneda en mayúsculas y con "USD"/"ARS".
func NormalizaMoneda(m string) string {
	m = strings.ToUpper(strings.TrimSpace(m))
	switch m {
	case "US", "U$S":
		return "USD"
	case "$":
		return "ARS"
	}
	return m
}
