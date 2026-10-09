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

// BattleNet intenta traer el precio de una URL de shop.battle.net.
// Hoy devuelve error: la tienda responde 401 (anti-bot) sin browser real.
func BattleNet(ctx context.Context, c *http.Client, url string) (PrecioTienda, error) {
	ua := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/122.0 Safari/537.36"
	resp, err := get(ctx, c, url, ua)
	if err != nil {
		return PrecioTienda{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return PrecioTienda{}, fmt.Errorf("battle.net respondió %d (anti-bot) en %s", resp.StatusCode, url)
	}
	// Aunque diera 200, el precio se arma en JS del cliente: haría falta browser.
	return PrecioTienda{}, fmt.Errorf("battle.net requiere browser headless (HTML sin precio embebido)")
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
