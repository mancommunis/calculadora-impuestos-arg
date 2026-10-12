// Package configuracion carga la config-as-code y expone los tipos que van al motor.
package configuracion

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type RegimenParams struct {
	IVA       float64 `json:"iva"`
	Ganancias float64 `json:"ganancias"`
}

type TaxConfig struct {
	ImpuestoPais float64                  `json:"impuesto_pais"`
	Regimenes    map[string]RegimenParams `json:"regimenes"`
	IIBB         map[string]float64       `json:"iibb"`
}

type Servicio struct {
	ID        string   `json:"id"`
	Nombre    string   `json:"nombre"`
	Grupo     string   `json:"grupo,omitempty"`
	Aliases   []string `json:"aliases"`
	Regimen   string   `json:"regimen"`
	Moneda    string   `json:"moneda"`
	Precio    *float64 `json:"precio"`
	Tienda    string   `json:"tienda,omitempty"`
	AppID     int      `json:"appid,omitempty"`
	URL       string   `json:"url,omitempty"`
	Edicion   string   `json:"edicion,omitempty"`
	OnDemand  bool     `json:"on_demand,omitempty"`
	Minimo    float64  `json:"minimo,omitempty"`
	IVAOrigen float64  `json:"iva_origen,omitempty"`
	Manual    bool     `json:"manual,omitempty"`
}

type Medio struct {
	ID             string   `json:"id"`
	Nombre         string   `json:"nombre"`
	Emisor         string   `json:"emisor"`
	Origen         string   `json:"origen"`
	Tipo           string   `json:"tipo"`
	Fx             string   `json:"fx"`
	Comision       float64  `json:"comision"`
	Cashback       float64  `json:"cashback"`
	SuscripcionUSD float64  `json:"suscripcion_usd"`
	AceptaEn       []string `json:"acepta_en"`
	Monedas        []string `json:"monedas"`
}

// Perfil es un comportamiento de medio reutilizable (banco local, global, etc.).
type Perfil struct {
	Tipo           string   `json:"tipo"`
	Origen         string   `json:"origen"`
	Fx             string   `json:"fx"`
	Comision       float64  `json:"comision"`
	Cashback       float64  `json:"cashback"`
	SuscripcionUSD float64  `json:"suscripcion_usd"`
	Monedas        []string `json:"monedas"`
	AceptaEn       []string `json:"acepta_en"`
}

// Emisor hereda un perfil y puede pisar fx/comision/cashback/suscripcion.
type Emisor struct {
	ID             string   `json:"id"`
	Nombre         string   `json:"nombre"`
	Perfil         string   `json:"perfil"`
	Fx             string   `json:"fx,omitempty"`
	Comision       *float64 `json:"comision,omitempty"`
	Cashback       *float64 `json:"cashback,omitempty"`
	SuscripcionUSD *float64 `json:"suscripcion_usd,omitempty"`
}

// Wallets es el archivo config/wallets.json (perfiles + emisores).
type Wallets struct {
	Perfiles map[string]Perfil `json:"perfiles"`
	Emisores []Emisor          `json:"emisores"`
}

// Expandir combina cada emisor con su perfil (y overrides) en medios concretos.
func (w Wallets) Expandir() []Medio {
	out := make([]Medio, 0, len(w.Emisores))
	for _, e := range w.Emisores {
		p := w.Perfiles[e.Perfil]
		origen := p.Origen
		if origen == "" {
			origen = p.Tipo
		}
		m := Medio{
			ID:             e.ID,
			Nombre:         e.Nombre,
			Emisor:         e.Nombre,
			Origen:         origen,
			Tipo:           p.Tipo,
			Fx:             p.Fx,
			Comision:       p.Comision,
			Cashback:       p.Cashback,
			SuscripcionUSD: p.SuscripcionUSD,
			Monedas:        p.Monedas,
			AceptaEn:       p.AceptaEn,
		}
		if e.Comision != nil {
			m.Comision = *e.Comision
		}
		if e.Fx != "" {
			m.Fx = e.Fx
		}
		if e.Cashback != nil {
			m.Cashback = *e.Cashback
		}
		if e.SuscripcionUSD != nil {
			m.SuscripcionUSD = *e.SuscripcionUSD
		}
		out = append(out, m)
	}
	return out
}

// Config es el conjunto de la config-as-code que consume la app.
type Config struct {
	Fiscal    TaxConfig
	Servicios []Servicio
	Medios    []Medio
}

// Cargar lee tax-config.json, subscriptions.json y wallets.json del directorio.
func Cargar(dir string) (Config, error) {
	var c Config
	if err := leer(filepath.Join(dir, "tax-config.json"), &c.Fiscal); err != nil {
		return c, err
	}
	var subs struct {
		Servicios []Servicio `json:"servicios"`
	}
	if err := leer(filepath.Join(dir, "subscriptions.json"), &subs); err != nil {
		return c, err
	}
	c.Servicios = subs.Servicios
	var wal Wallets
	if err := leer(filepath.Join(dir, "wallets.json"), &wal); err != nil {
		return c, err
	}
	c.Medios = wal.Expandir()

	// Catálogos (dataset/*.json), si existen: se suman a los servicios.
	for _, f := range []string{"catalogo.json", "battlenet.json"} {
		p := filepath.Join(filepath.Dir(dir), "dataset", f)
		if cat, err := CargarCatalogo(p); err == nil {
			c.Servicios = append(c.Servicios, cat.Productos...)
		}
	}
	return c, nil
}

// Catalogo es el archivo dataset/catalogo.json (juegos + precios por tienda).
type Catalogo struct {
	Productos []Servicio `json:"productos"`
}

func CargarCatalogo(path string) (Catalogo, error) {
	var c Catalogo
	if err := leer(path, &c); err != nil {
		return c, err
	}
	return c, nil
}

func GuardarCatalogo(path string, c Catalogo) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// DirPorDefecto ubica el directorio config/ subiendo desde el cwd.
func DirPorDefecto() string {
	if p := os.Getenv("MANCOMMUNIS_CONFIG"); p != "" {
		return p
	}
	dir, err := os.Getwd()
	if err != nil {
		return "config"
	}
	for i := 0; i < 8; i++ {
		cand := filepath.Join(dir, "config")
		if _, err := os.Stat(filepath.Join(cand, "tax-config.json")); err == nil {
			return cand
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			break
		}
		dir = padre
	}
	return "config"
}

func leer(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func (c Config) Provincias() []string {
	out := make([]string, 0, len(c.Fiscal.IIBB))
	for k := range c.Fiscal.IIBB {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ParametrosFiscales es lo que se le pasa al motor (régimen + provincia ya resueltos).
type ParametrosFiscales struct {
	IVA          float64 `json:"iva"`
	Ganancias    float64 `json:"ganancias"`
	IIBB         float64 `json:"iibb"`
	ImpuestoPais float64 `json:"impuesto_pais"`
}

func (c Config) Parametros(regimen, provincia string) (ParametrosFiscales, error) {
	r, ok := c.Fiscal.Regimenes[regimen]
	if !ok {
		return ParametrosFiscales{}, fmt.Errorf("régimen desconocido: %s", regimen)
	}
	iibb, ok := c.Fiscal.IIBB[provincia]
	if !ok {
		return ParametrosFiscales{}, fmt.Errorf("provincia sin IIBB: %s", provincia)
	}
	return ParametrosFiscales{IVA: r.IVA, Ganancias: r.Ganancias, IIBB: iibb, ImpuestoPais: c.Fiscal.ImpuestoPais}, nil
}

// AplicaA indica si el medio sirve para una tienda y una moneda del precio.
func (m Medio) AplicaA(tienda, moneda string) bool {
	tiendaOK := len(m.AceptaEn) == 0
	for _, t := range m.AceptaEn {
		if t == "*" || (tienda != "" && t == tienda) {
			tiendaOK = true
			break
		}
	}
	if !tiendaOK {
		return false
	}
	if len(m.Monedas) == 0 {
		return true
	}
	for _, mo := range m.Monedas {
		if strings.EqualFold(mo, moneda) {
			return true
		}
	}
	return false
}

// Entrada es el JSON que se manda al motor.
type Entrada struct {
	Precio       Precio             `json:"precio"`
	Regimen      string             `json:"regimen"`
	Parametros   ParametrosFiscales `json:"parametros"`
	Medio        MedioEntrada       `json:"medio"`
	Cotizaciones map[string]float64 `json:"cotizaciones"`
}

type Precio struct {
	Moneda string  `json:"moneda"`
	Monto  float64 `json:"monto"`
}

type MedioEntrada struct {
	Nombre       string  `json:"nombre"`
	Tipo         string  `json:"tipo"`
	Fx           string  `json:"fx"`
	RecargoExtra float64 `json:"recargo_extra"`
	Cashback     float64 `json:"cashback"`
}

// Desglose es lo que devuelve el motor.
type Desglose struct {
	Regimen      string  `json:"regimen"`
	MonedaPrecio string  `json:"moneda_precio"`
	MontoPrecio  float64 `json:"monto_precio"`
	FxNombre     string  `json:"fx_nombre"`
	FxValor      float64 `json:"fx_valor"`
	BaseARS      float64 `json:"base_ars"`
	IVA          float64 `json:"iva"`
	IIBB         float64 `json:"iibb"`
	Ganancias    float64 `json:"ganancias"`
	Cashback     float64 `json:"cashback"`
	TotalARS     float64 `json:"total_ars"`
	TasaEfectiva float64 `json:"tasa_efectiva"`
}

// Buscar hace match por nombre + aliases (substring, case-insensitive).
func Buscar(servicios []Servicio, q string) []Servicio {
	t := strings.ToLower(strings.TrimSpace(q))
	if t == "" {
		return nil
	}
	type punt struct {
		s     Servicio
		score int
	}
	var ps []punt
	for _, s := range servicios {
		campos := append([]string{s.Nombre, s.ID}, s.Aliases...)
		score := 0
		for _, c := range campos {
			cl := strings.ToLower(c)
			switch {
			case cl == t:
				score = max(score, 100)
			case strings.HasPrefix(cl, t):
				score = max(score, 60)
			case strings.Contains(cl, t):
				score = max(score, 30)
			}
		}
		if score > 0 {
			ps = append(ps, punt{s, score})
		}
	}
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].score > ps[j].score })
	out := make([]Servicio, len(ps))
	for i, p := range ps {
		out[i] = p.s
	}
	return out
}

// Grupo anida variantes de una misma franquicia.
type Grupo struct {
	Clave  string
	Nombre string
	Items  []Servicio
}

// BuscarGrupos busca y anida por `grupo` (o por id si no tiene).
func BuscarGrupos(servicios []Servicio, q string) []Grupo {
	items := Buscar(servicios, q)
	var out []Grupo
	idx := map[string]int{}
	for _, s := range items {
		clave := s.Grupo
		nombre := s.Grupo
		if clave == "" {
			clave = s.ID
			nombre = s.Nombre
		}
		if i, ok := idx[clave]; ok {
			out[i].Items = append(out[i].Items, s)
		} else {
			idx[clave] = len(out)
			out = append(out, Grupo{Clave: clave, Nombre: nombre, Items: []Servicio{s}})
		}
	}
	return out
}
