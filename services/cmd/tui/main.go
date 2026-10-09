// Comando tui: la calculadora en la consola (Bubble Tea).
//
// Escribís un juego/servicio, ves el desglose fiscal por medio de pago.
// El motor corre embebido (mismo WASM que la web y la API).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/configuracion"
	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/motor"
)

// ---- estilos (fósforo) ----

var (
	fondo   = lipgloss.Color("#0B0F0C")
	fosforo = lipgloss.Color("#63F5A1")
	tenue   = lipgloss.Color("#2E7D57")
	cian    = lipgloss.Color("#4CE0E0")
	magenta = lipgloss.Color("#FF5CA8")
	texto   = lipgloss.Color("#CFEEDE")

	estTitulo  = lipgloss.NewStyle().Foreground(fosforo).Bold(true)
	estSub     = lipgloss.NewStyle().Foreground(tenue)
	estTenue   = lipgloss.NewStyle().Foreground(tenue)
	estCian    = lipgloss.NewStyle().Foreground(cian)
	estMagenta = lipgloss.NewStyle().Foreground(magenta)
	estTexto   = lipgloss.NewStyle().Foreground(texto)
	estSel     = lipgloss.NewStyle().Foreground(fondo).Background(fosforo).Bold(true)
	estPunto   = lipgloss.NewStyle().Foreground(fosforo)
)

// ---- modelo ----

type calculo struct {
	medio    configuracion.Medio
	desglose configuracion.Desglose
}

type cargadoMsg struct {
	m          *motor.Motor
	cfg        configuracion.Config
	fx         map[string]float64
	provincias []string
	err        error
}

type modelo struct {
	ctx   context.Context
	listo bool
	err   error

	motor      *motor.Motor
	cfg        configuracion.Config
	provincias []string
	fx         map[string]float64
	provincia  string

	input       textinput.Model
	resultados  []configuracion.Servicio
	cursor      int
	scrollLista int

	seleccion *configuracion.Servicio
	monto     float64
	calculos  []calculo
	scrollDet int

	alto  int
	ancho int
}

// ---- carga inicial ----

func cargar() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		m, err := motor.Nuevo(ctx, motor.RutaWasmPorDefecto())
		if err != nil {
			return cargadoMsg{err: err}
		}
		cfg, err := configuracion.Cargar(configuracion.DirPorDefecto())
		if err != nil {
			return cargadoMsg{err: err}
		}
		fx, _ := traerFx(ctx)
		return cargadoMsg{m: m, cfg: cfg, fx: fx, provincias: cfg.Provincias()}
	}
}

func traerFx(ctx context.Context) (map[string]float64, error) {
	c := &http.Client{Timeout: 10 * time.Second}
	campo := func(u, campo string) (float64, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return 0, err
		}
		resp, err := c.Do(req)
		if err != nil {
			return 0, err
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			return 0, err
		}
		switch v := m[campo].(type) {
		case float64:
			return v, nil
		case string:
			return strconv.ParseFloat(strings.TrimSpace(v), 64)
		}
		return 0, fmt.Errorf("campo %q ausente en %s", campo, u)
	}

	fx := map[string]float64{}
	for clave, url := range map[string]string{
		"oficial": "https://dolarapi.com/v1/dolares/oficial",
		"mep":     "https://dolarapi.com/v1/dolares/bolsa",
		"cripto":  "https://dolarapi.com/v1/dolares/cripto",
		"ccl":     "https://dolarapi.com/v1/dolares/contadoconliqui",
	} {
		if v, err := campo(url, "venta"); err == nil && v > 0 {
			fx[clave] = v
		}
	}
	exchanges := []string{"ripio", "belo", "buenbit", "lemon"}
	for _, ex := range exchanges {
		if v, err := campo("https://criptoya.com/api/"+ex+"/usdt/ars/1", "ask"); err == nil && v > 0 {
			fx[ex] = v
		}
	}
	for _, ex := range exchanges {
		if _, ok := fx[ex]; !ok {
			if cr, ok := fx["cripto"]; ok {
				fx[ex] = cr
			}
		}
	}
	if len(fx) == 0 {
		return nil, fmt.Errorf("sin cotizaciones disponibles")
	}
	return fx, nil
}

// ---- bubble tea ----

func inicial() modelo {
	in := textinput.New()
	in.Placeholder = "buscá un juego o servicio… (ej. wow, cyberpunk, deepseek)"
	in.Focus()
	in.CharLimit = 60
	return modelo{ctx: context.Background(), input: in, provincia: "CABA", fx: map[string]float64{}, alto: 30, ancho: 80}
}

func (m modelo) Init() tea.Cmd { return tea.Batch(textinput.Blink, cargar()) }

func (m modelo) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.alto = msg.Height
		m.ancho = msg.Width
		return m, nil
	case cargadoMsg:
		if msg.err != nil {
			m.err = msg.err
			m.listo = true
			return m, nil
		}
		m.motor = msg.m
		m.cfg = msg.cfg
		m.fx = msg.fx
		m.provincias = msg.provincias
		if m.provincia == "" && len(m.provincias) > 0 {
			m.provincia = m.provincias[0]
		}
		m.listo = true
		return m, nil
	case tea.MouseMsg:
		m.scrollear(msg)
		return m, nil
	case tea.KeyMsg:
		if m.seleccion != nil {
			return m.updateDetalle(msg)
		}
		return m.updateBuscar(msg)
	}
	return m, nil
}

// scrollear maneja la rueda del mouse (scroll interno, no el de la terminal).
func (m *modelo) scrollear(msg tea.MouseMsg) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.mover(-1)
	case tea.MouseButtonWheelDown:
		m.mover(1)
	}
}

func (m *modelo) mover(delta int) {
	if m.seleccion != nil {
		m.scrollDet = clamp(m.scrollDet+delta, 0, max(0, len(m.calculos)-1))
		return
	}
	m.cursor = clamp(m.cursor+delta, 0, max(0, len(m.resultados)-1))
	m.ajustarScrollLista()
}

func (m *modelo) ajustarScrollLista() {
	vis := m.visibles()
	if m.cursor < m.scrollLista {
		m.scrollLista = m.cursor
	}
	if m.cursor >= m.scrollLista+vis {
		m.scrollLista = m.cursor - vis + 1
	}
}

func (m modelo) visibles() int {
	if m.alto > 14 {
		return m.alto - 8
	}
	return 6
}

func (m modelo) updateBuscar(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		m.mover(-1)
		return m, nil
	case "down", "j":
		m.mover(1)
		return m, nil
	case "enter":
		if len(m.resultados) > 0 {
			return m.seleccionar(m.resultados[m.cursor])
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.resultados = configuracion.Buscar(m.cfg.Servicios, m.input.Value())
	m.cursor = 0
	m.scrollLista = 0
	return m, cmd
}

func (m modelo) updateDetalle(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.seleccion = nil
		m.calculos = nil
		m.scrollDet = 0
		return m, nil
	case "p":
		m.provincia = m.siguienteProvincia()
		m.recalcular()
		return m, nil
	case "r":
		if fx, err := traerFx(m.ctx); err == nil {
			m.fx = fx
		}
		m.recalcular()
		return m, nil
	case "up", "k":
		m.mover(-1)
		return m, nil
	case "down", "j":
		m.mover(1)
		return m, nil
	case "pgup":
		m.mover(-m.visibles())
		return m, nil
	case "pgdown":
		m.mover(m.visibles())
		return m, nil
	case "+", "right":
		m.monto += m.paso()
		m.recalcular()
		return m, nil
	case "-", "left":
		if m.monto > m.paso() {
			m.monto -= m.paso()
		}
		m.recalcular()
		return m, nil
	}
	return m, nil
}

func (m modelo) siguienteProvincia() string {
	for i, p := range m.provincias {
		if p == m.provincia {
			return m.provincias[(i+1)%len(m.provincias)]
		}
	}
	if len(m.provincias) > 0 {
		return m.provincias[0]
	}
	return m.provincia
}

func (m modelo) paso() float64 {
	if m.seleccion != nil && strings.EqualFold(m.seleccion.Moneda, "USD") {
		return 1
	}
	return 500
}

func (m modelo) seleccionar(s configuracion.Servicio) (tea.Model, tea.Cmd) {
	m.seleccion = &s
	if s.Precio != nil {
		m.monto = *s.Precio
	} else if strings.EqualFold(s.Moneda, "USD") {
		m.monto = 10
	} else {
		m.monto = 5000
	}
	m.scrollDet = 0
	m.scrollLista = 0
	m.recalcular()
	return m, nil
}

func (m *modelo) recalcular() {
	m.calculos = nil
	if m.seleccion == nil || m.motor == nil {
		return
	}
	p, err := m.cfg.Parametros(m.seleccion.Regimen, m.provincia)
	if err != nil {
		m.err = err
		return
	}
	monto := m.monto * (1 + m.seleccion.IVAOrigen)
	for _, medio := range m.cfg.Medios {
		if !medio.AplicaA(m.seleccion.Tienda, m.seleccion.Moneda) {
			continue
		}
		entrada := configuracion.Entrada{
			Precio:       configuracion.Precio{Moneda: strings.ToLower(m.seleccion.Moneda), Monto: monto},
			Regimen:      m.seleccion.Regimen,
			Parametros:   p,
			Medio:        configuracion.MedioEntrada{Nombre: medio.Nombre, Tipo: medio.Tipo, Fx: medio.Fx, RecargoExtra: medio.Comision, Cashback: medio.Cashback},
			Cotizaciones: m.fx,
		}
		b, _ := json.Marshal(entrada)
		salida, err := m.motor.CalcularJSON(m.ctx, string(b))
		if err != nil {
			continue
		}
		// El motor devuelve {"error": ...} ante problemas (ej. falta cotización):
		// NO es un Desglose en cero, hay que descartarlo.
		var r struct {
			configuracion.Desglose
			Error string `json:"error"`
		}
		if json.Unmarshal([]byte(salida), &r) != nil || r.Error != "" {
			continue
		}
		m.calculos = append(m.calculos, calculo{medio: medio, desglose: r.Desglose})
	}
	for i := 1; i < len(m.calculos); i++ {
		for j := i; j > 0 && m.calculos[j].desglose.TotalARS < m.calculos[j-1].desglose.TotalARS; j-- {
			m.calculos[j], m.calculos[j-1] = m.calculos[j-1], m.calculos[j]
		}
	}
}

// ---- render ----

func (m modelo) View() string {
	if !m.listo {
		return estSub.Render("cargando motor fiscal…") + "\n"
	}
	if m.err != nil {
		return estMagenta.Render("error: "+m.err.Error()) + "\n"
	}
	if m.seleccion != nil {
		return m.viewDetalle()
	}
	return m.viewBuscar()
}

func (m modelo) viewBuscar() string {
	var b strings.Builder
	b.WriteString(estTitulo.Render("calculadora") + estMagenta.Render(".") + estTitulo.Render("impuestos") + estMagenta.Render(".") + estTitulo.Render("arg") + "\n")
	b.WriteString(estSub.Render("precio final de compras digitales, con impuestos") + "\n\n")
	b.WriteString(m.input.View() + "\n\n")

	if m.input.Value() != "" && len(m.resultados) == 0 {
		b.WriteString(estTenue.Render("sin resultados") + "\n")
	}

	vis := m.visibles()
	fin := min(len(m.resultados), m.scrollLista+vis)
	for i := m.scrollLista; i < fin; i++ {
		s := m.resultados[i]
		meta := estTenue.Render(s.Tienda + " · " + monedaPrecio(s))
		if s.Tienda == "" {
			meta = estTenue.Render(s.Regimen + " · " + monedaPrecio(s))
		}
		if i == m.cursor {
			b.WriteString(estSel.Render("▸ "+s.Nombre) + "  " + meta + "\n")
		} else {
			b.WriteString("  " + estTexto.Render(s.Nombre) + "  " + meta + "\n")
		}
	}
	if len(m.resultados) > vis {
		b.WriteString(estTenue.Render(fmt.Sprintf("  (%d-%d de %d · ↑↓ scrollea)", m.scrollLista+1, fin, len(m.resultados))) + "\n")
	}
	b.WriteString("\n" + estTenue.Render("escribí · ↑↓ elegir · enter ver · q salir") + "\n")
	return b.String()
}

func (m modelo) viewDetalle() string {
	s := m.seleccion
	var b strings.Builder
	b.WriteString(estTitulo.Render("▓ "+s.Nombre) + "  " + estCian.Render(s.Regimen) + "\n")

	precio := fmt.Sprintf("%s %s", formatoNum(m.monto), s.Moneda)
	if s.IVAOrigen > 0 {
		precio += fmt.Sprintf(" +%.0f%% VAT = %s", s.IVAOrigen*100, formatoNum((m.monto)*(1+s.IVAOrigen)))
	}
	b.WriteString(estSub.Render(fmt.Sprintf("%s · %s", precio, m.provincia)) + "\n")

	var fxParts []string
	for _, k := range []string{"oficial", "mep", "cripto"} {
		if v, ok := m.fx[k]; ok {
			fxParts = append(fxParts, fmt.Sprintf("%s %s", k, formatoNum(v)))
		}
	}
	if len(fxParts) > 0 {
		b.WriteString(estTenue.Render("dólar: "+strings.Join(fxParts, " · ")) + "\n")
	}
	b.WriteString("\n")

	if len(m.calculos) == 0 {
		b.WriteString(estTenue.Render("sin medios aplicables / falta cotización") + "\n")
	}

	vis := m.visibles()
	m.scrollDet = clamp(m.scrollDet, 0, max(0, len(m.calculos)-1))
	fin := min(len(m.calculos), m.scrollDet+vis)
	for i := m.scrollDet; i < fin; i++ {
		b.WriteString(m.lineaMedio(i, i == 0) + "\n")
	}
	if len(m.calculos) > vis {
		b.WriteString(estTenue.Render(fmt.Sprintf("  (%d-%d de %d · ↑↓ scrollea)", m.scrollDet+1, fin, len(m.calculos))) + "\n")
	}

	if len(m.calculos) > 0 {
		b.WriteString("\n" + estTitulo.Render("mejor hoy: "+nombreMedio(m.calculos[0].medio)) + "\n")
	}
	b.WriteString("\n" + estTenue.Render("↑↓ scrollear · p provincia · r dólar · +/- monto · esc volver · q salir") + "\n")
	return b.String()
}

func (m modelo) lineaMedio(i int, mejor bool) string {
	c := m.calculos[i]
	nombre := truncar(nombreMedio(c.medio), 26)
	pad := nombre + strings.Repeat(" ", max(0, 26-len([]rune(nombre))))
	porc := fmt.Sprintf("%+.1f%%", c.desglose.TasaEfectiva*100)
	total := formatoPesos(c.desglose.TotalARS)
	if mejor {
		return estPunto.Render("▸ "+pad) + " " + estCian.Render(fmt.Sprintf("%14s", total)) + " " + estTenue.Render(fmt.Sprintf("%8s", porc))
	}
	return estTexto.Render("  "+pad) + " " + estTexto.Render(fmt.Sprintf("%14s", total)) + " " + estTenue.Render(fmt.Sprintf("%8s", porc))
}

func main() {
	flag.Parse()
	p := tea.NewProgram(inicial(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

// ---- helpers ----

func monedaPrecio(s configuracion.Servicio) string {
	if s.Precio != nil {
		return fmt.Sprintf("%s %s", formatoNum(*s.Precio), s.Moneda)
	}
	return s.Moneda
}

func formatoNum(f float64) string {
	entero := int64(f)
	neg := f < 0
	if neg {
		entero = -entero
	}
	dec := ""
	if frac := math.Abs(f - float64(int64(f))); frac > 0.0001 {
		dec = fmt.Sprintf(",%02d", int64(frac*100+0.5))
	}
	s := fmt.Sprintf("%d", entero)
	var partes []string
	for len(s) > 3 {
		partes = append([]string{s[len(s)-3:]}, partes...)
		s = s[:len(s)-3]
	}
	partes = append([]string{s}, partes...)
	out := strings.Join(partes, ".")
	if neg {
		out = "-" + out
	}
	return out + dec
}

func formatoPesos(f float64) string {
	return "$ " + formatoNum(float64(int64(f+0.5)))
}

func nombreMedio(md configuracion.Medio) string {
	if md.Nombre != "" {
		return md.Nombre
	}
	return md.ID
}

func truncar(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
