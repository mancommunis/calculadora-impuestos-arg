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
	estPuntito = lipgloss.NewStyle().Foreground(fosforo)
	estCaja    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(tenue).Padding(0, 1)
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

	input      textinput.Model
	resultados []configuracion.Servicio
	cursor     int

	seleccion *configuracion.Servicio
	monto     float64
	calculos  []calculo
	ancho     int
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
		fx, _ := traerFx(ctx) // best-effort; si falla, quedan vacíos
		return cargadoMsg{m: m, cfg: cfg, fx: fx, provincias: cfg.Provincias()}
	}
}

func traerFx(ctx context.Context) (map[string]float64, error) {
	c := &http.Client{Timeout: 10 * time.Second}
	venta := func(u string) (float64, error) {
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
		var out struct {
			Venta float64 `json:"venta"`
		}
		if err := json.Unmarshal(b, &out); err != nil {
			return 0, err
		}
		return out.Venta, nil
	}
	of, err := venta("https://dolarapi.com/v1/dolares/oficial")
	if err != nil {
		return nil, err
	}
	mep, err := venta("https://dolarapi.com/v1/dolares/bolsa")
	if err != nil {
		return map[string]float64{"oficial": of}, nil
	}
	return map[string]float64{"oficial": of, "mep": mep}, nil
}

// ---- bubble tea ----

func inicial() modelo {
	in := textinput.New()
	in.Placeholder = "buscá un juego o servicio… (ej. WoW Forever, ChatGPT)"
	in.Focus()
	in.CharLimit = 60
	return modelo{ctx: context.Background(), input: in, provincia: "CABA", fx: map[string]float64{}}
}

func (m modelo) Init() tea.Cmd { return tea.Batch(textinput.Blink, cargar()) }

func (m modelo) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
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

	case tea.KeyMsg:
		if m.seleccion != nil {
			return m.updateDetalle(msg)
		}
		return m.updateBuscar(msg)
	}
	return m, nil
}

func (m modelo) updateBuscar(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "shift+tab":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "tab":
		if m.cursor < len(m.resultados)-1 {
			m.cursor++
		}
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
	if m.cursor >= len(m.resultados) {
		m.cursor = 0
	}
	return m, cmd
}

func (m modelo) updateDetalle(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.seleccion = nil
		m.calculos = nil
		return m, nil
	case "tab", "p":
		m.provincia = m.siguienteProvincia()
		m.recalcular()
		return m, nil
	case "r":
		if fx, err := traerFx(m.ctx); err == nil {
			m.fx = fx
		}
		m.recalcular()
		return m, nil
	case "+", "up":
		m.monto += m.paso()
		m.recalcular()
		return m, nil
	case "-", "down":
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
	for _, medio := range m.cfg.Medios {
		if !medio.AplicaA(m.seleccion.Tienda) {
			continue
		}
		entrada := configuracion.Entrada{
			Precio:       configuracion.Precio{Moneda: strings.ToLower(m.seleccion.Moneda), Monto: m.monto},
			Regimen:      m.seleccion.Regimen,
			Parametros:   p,
			Medio:        configuracion.MedioEntrada{Nombre: medio.ID, Tipo: medio.Tipo, Fx: medio.Fx, RecargoExtra: medio.RecargoExtra},
			Cotizaciones: m.fx,
		}
		b, _ := json.Marshal(entrada)
		salida, err := m.motor.CalcularJSON(m.ctx, string(b))
		if err != nil {
			continue
		}
		var d configuracion.Desglose
		if json.Unmarshal([]byte(salida), &d) == nil {
			m.calculos = append(m.calculos, calculo{medio: medio, desglose: d})
		}
	}
	// ordenar por total ascendente (el más barato primero)
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
	for i, s := range m.resultados {
		meta := estTenue.Render(s.Regimen + " · " + s.Moneda)
		if i == m.cursor {
			b.WriteString(estSel.Render("▸ "+s.Nombre) + "  " + meta + "\n")
		} else {
			b.WriteString("  " + estTexto.Render(s.Nombre) + "  " + meta + "\n")
		}
	}
	b.WriteString("\n" + estTenue.Render("↑↓ elegir · enter ver · q salir") + "\n")
	return b.String()
}

func (m modelo) viewDetalle() string {
	var b strings.Builder
	s := m.seleccion
	b.WriteString(estTitulo.Render("▓ "+s.Nombre) + "  " + estCian.Render(s.Regimen) + "\n")
	b.WriteString(estSub.Render(fmt.Sprintf("lista %s %s · monto %s · provincia %s",
		formatoNum(m.monto), s.Moneda, formatoNum(m.monto), m.provincia)) + "\n\n")

	// cotizaciones
	var fxParts []string
	for _, k := range []string{"oficial", "mep"} {
		if v, ok := m.fx[k]; ok {
			fxParts = append(fxParts, fmt.Sprintf("%s %s", k, formatoNum(v)))
		}
	}
	if len(fxParts) > 0 {
		b.WriteString(estTenue.Render("dólar: "+strings.Join(fxParts, " · ")) + "\n\n")
	}

	if len(m.calculos) == 0 {
		b.WriteString(estTenue.Render("sin medios aplicables / falta cotización") + "\n")
	}

	for i, c := range m.calculos {
		marca := "  "
		if i == 0 {
			marca = estPuntito.Render("▸ ")
		}
		cab := marca + estTexto.Render(c.medio.ID) + "  " + estTenue.Render(c.medio.Tipo)
		detalle := estTenue.Render(fmt.Sprintf("IVA %s · IIBB %s · Ganancias %s",
			formatoNum(c.desglose.IVA), formatoNum(c.desglose.IIBB), formatoNum(c.desglose.Ganancias)))
		total := estCian.Render(formatoPesos(c.desglose.TotalARS)) + estTenue.Render(fmt.Sprintf("  (+%.1f%%)", c.desglose.TasaEfectiva*100))
		if i == 0 {
			total = estTitulo.Render(formatoPesos(c.desglose.TotalARS)) + estTenue.Render(fmt.Sprintf("  (+%.1f%%)", c.desglose.TasaEfectiva*100))
		}
		b.WriteString(cab + "\n    " + detalle + "\n    " + total + "\n\n")
	}

	if len(m.calculos) > 0 {
		b.WriteString(estTitulo.Render("mejor hoy: "+m.calculos[0].medio.ID) + "\n")
	}
	b.WriteString("\n" + estTenue.Render("p provincia · r refrescar dólar · +/- monto · esc volver · q salir") + "\n")
	return estCaja.Render(strings.TrimRight(b.String(), "\n"))
}

func main() {
	flag.Parse()
	p := tea.NewProgram(inicial(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

// ---- formatos ----

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
