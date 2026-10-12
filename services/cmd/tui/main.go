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
	provincia  string
	err        error
}

// fila es una fila visible de la búsqueda (grupo / subgrupo / juego / ítem).
// tipo: 0 = grupo, 1 = subgrupo, 2 = juego, 3 = ítem.
type fila struct {
	tipo   int
	clave  string
	sub    string
	nombre string
	n      int
	item   configuracion.Servicio
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
	filas       []fila
	expandido   map[string]bool
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
		return cargadoMsg{m: m, cfg: cfg, fx: fx, provincias: cfg.Provincias(), provincia: detectarProvincia(ctx, cfg.Provincias())}
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
		return parsearCampo(b, campo)
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
	return modelo{ctx: context.Background(), input: in, provincia: "CABA", fx: map[string]float64{}, filas: []fila{}, expandido: map[string]bool{}, alto: 30, ancho: 80}
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
		if msg.provincia != "" {
			m.provincia = msg.provincia
		} else if m.provincia == "" && len(m.provincias) > 0 {
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
		m.scrollDet = clamp(m.scrollDet+delta, 0, max(0, len(m.calculos)-m.visibles()))
		return
	}
	m.cursor = clamp(m.cursor+delta, 0, max(0, len(m.filas)-1))
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
	// Frame = header (4) + vista (N) + footer (hasta 6). Reservamos 12 para que
	// SIEMPRE entre en la pantalla: header y footer quedan fijos.
	if m.alto > 16 {
		return m.alto - 12
	}
	return 4
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
		if m.cursor >= 0 && m.cursor < len(m.filas) {
			f := m.filas[m.cursor]
			switch f.tipo {
			case 0:
				m.expandido["g:"+f.clave] = !m.expandido["g:"+f.clave]
				m.actualizarFilas()
				return m, nil
			case 1:
				k := "s:" + f.clave + "|" + f.sub
				m.expandido[k] = !m.expandido[k]
				m.actualizarFilas()
				return m, nil
			case 2:
				return m, nil // juego: encabezado, no seleccionable
			default:
				return m.seleccionar(f.item)
			}
		}
		return m, nil
	case "esc":
		m.input.SetValue("")
		m.cursor, m.scrollLista = 0, 0
		m.actualizarFilas()
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.cursor, m.scrollLista = 0, 0
	m.actualizarFilas()
	return m, cmd
}

func (m modelo) updateDetalle(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ajuste de monto (solo on-demand): acepta +/= y -/_, más flechas.
	if m.esOnDemand() {
		if teclaSuma(msg) {
			m.monto += m.paso()
			m.recalcular()
			return m, nil
		}
		if teclaResta(msg) {
			if m.monto-m.paso() >= m.piso() {
				m.monto -= m.paso()
			} else {
				m.monto = m.piso()
			}
			m.recalcular()
			return m, nil
		}
	}
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

// esOnDemand: el medio es prepago variable (ej. DeepSeek) -> monto ajustable.
func (m modelo) esOnDemand() bool {
	return m.seleccion != nil && m.seleccion.OnDemand
}

// piso: monto mínimo permitido (ej. DeepSeek US$2).
func (m modelo) piso() float64 {
	if m.seleccion == nil {
		return 0
	}
	if m.seleccion.Minimo > 0 {
		return m.seleccion.Minimo
	}
	if m.seleccion.Precio != nil {
		return *m.seleccion.Precio
	}
	return 0
}

// teclaSuma: + , = (Shift+= en Mac) o flecha derecha.
func teclaSuma(msg tea.KeyMsg) bool {
	if msg.Type == tea.KeyRight {
		return true
	}
	for _, r := range msg.Runes {
		if r == '+' || r == '=' {
			return true
		}
	}
	return false
}

// teclaResta: - o flecha izquierda.
func teclaResta(msg tea.KeyMsg) bool {
	if msg.Type == tea.KeyLeft {
		return true
	}
	for _, r := range msg.Runes {
		if r == '-' || r == '_' {
			return true
		}
	}
	return false
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
	if p := m.piso(); m.monto < p {
		m.monto = p
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
	vis := m.visibles()
	m.scrollLista = clamp(m.scrollLista, 0, max(0, len(m.filas)-vis))

	var b strings.Builder
	// header fijo (4 líneas)
	b.WriteString(estTitulo.Render("calculadora") + estMagenta.Render(".") + estTitulo.Render("impuestos") + estMagenta.Render(".") + estTitulo.Render("arg") + "\n")
	b.WriteString(estSub.Render("precio final de compras digitales, con impuestos") + "\n")
	b.WriteString(m.input.View() + "\n")
	b.WriteString("\n")

	// viewport: exactamente 'vis' líneas
	for k := 0; k < vis; k++ {
		i := m.scrollLista + k
		switch {
		case i < len(m.filas):
			b.WriteString(m.lineaFila(i) + "\n")
		case k == 0 && m.input.Value() != "":
			b.WriteString(estTenue.Render("  sin resultados") + "\n")
		default:
			b.WriteString("\n")
		}
	}

	// footer fijo (3 líneas)
	b.WriteString("\n")
	if len(m.filas) > vis {
		b.WriteString(estTenue.Render(fmt.Sprintf("  %d-%d de %d", m.scrollLista+1, min(len(m.filas), m.scrollLista+vis), len(m.filas))) + "\n")
	} else {
		b.WriteString("\n")
	}
	b.WriteString(estTenue.Render("escribí · ↑↓ elegir · enter ver/abrir · q salir") + "\n")
	return strings.TrimRight(b.String(), "\n")
}

func (m modelo) lineaFila(i int) string {
	f := m.filas[i]
	var nombre, meta string
	switch f.tipo {
	case 0:
		fl := "▸"
		if m.expandido["g:"+f.clave] {
			fl = "▾"
		}
		nombre = fl + " " + f.nombre
		meta = fmt.Sprintf("%d items", f.n)
	case 1:
		fl := "▸"
		if m.expandido["s:"+f.clave+"|"+f.sub] {
			fl = "▾"
		}
		sub := f.nombre
		if sub == "" {
			sub = "Otros"
		}
		nombre = "  " + fl + " " + sub
		meta = fmt.Sprintf("%d", f.n)
	case 2:
		nombre = "    " + f.nombre
	case 3:
		tienda := nombreTienda(f.item.Tienda)
		if f.sub != "" && tienda != "" {
			nombre = "      " + tienda + " → " + f.sub
			meta = monedaPrecio(f.item)
		} else {
			if tienda == "" {
				tienda = f.item.Regimen
			}
			nombre = "    " + f.nombre
			meta = tienda + " · " + monedaPrecio(f.item)
		}
	}
	if i == m.cursor {
		return estSel.Render(nombre + "  " + meta)
	}
	if f.tipo == 2 {
		return estCian.Render(nombre)
	}
	return estTexto.Render(nombre) + "  " + estTenue.Render(meta)
}

func nombreTienda(t string) string {
	switch t {
	case "battle_net":
		return "Battle.net"
	case "steam":
		return "Steam"
	}
	return t
}

func (m modelo) viewDetalle() string {
	s := m.seleccion
	vis := m.visibles()
	m.scrollDet = clamp(m.scrollDet, 0, max(0, len(m.calculos)-vis))

	var b strings.Builder
	// header fijo (4 líneas)
	b.WriteString(estTitulo.Render("▓ "+s.Nombre) + "  " + estCian.Render(s.Regimen) + "\n")
	montoStr := fmt.Sprintf("%s %s", formatoNum(m.monto), s.Moneda)
	if s.IVAOrigen > 0 {
		montoStr += fmt.Sprintf("   +%.0f%% VAT → %s %s", s.IVAOrigen*100, formatoNum(m.monto*(1+s.IVAOrigen)), s.Moneda)
	}
	b.WriteString(estCian.Render("monto: ") + estTitulo.Render(montoStr) + estSub.Render("  ·  "+m.provincia) + "\n")
	var fxParts []string
	for _, k := range []string{"oficial", "mep", "cripto"} {
		if v, ok := m.fx[k]; ok {
			fxParts = append(fxParts, fmt.Sprintf("%s %s", k, formatoNum(v)))
		}
	}
	if len(fxParts) > 0 {
		b.WriteString(estTenue.Render("dólar: "+strings.Join(fxParts, " · ")) + "\n")
	} else {
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// viewport: exactamente 'vis' líneas (el ▸ marca TODOS los empatados en el mejor)
	best := mejoresIDs(m.calculos)
	for k := 0; k < vis; k++ {
		i := m.scrollDet + k
		switch {
		case i < len(m.calculos):
			b.WriteString(m.lineaMedio(i, best[m.calculos[i].medio.ID]) + "\n")
		case k == 0 && len(m.calculos) == 0:
			b.WriteString(estTenue.Render("  sin medios aplicables / falta cotización") + "\n")
		default:
			b.WriteString("\n")
		}
	}

	// footer fijo (6 líneas): contador, blank, mejores, blank, monto(on-demand), teclas
	if len(m.calculos) > vis {
		b.WriteString(estTenue.Render(fmt.Sprintf("  %d-%d de %d", m.scrollDet+1, min(len(m.calculos), m.scrollDet+vis), len(m.calculos))) + "\n")
	} else {
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if len(m.calculos) > 0 {
		b.WriteString(estTitulo.Render("los mejores hoy: "+mejoresNombres(m.calculos, 4)) + "\n")
	} else {
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if m.esOnDemand() {
		b.WriteString(estCian.Render(fmt.Sprintf("monto: %s %s", formatoNum(m.monto), s.Moneda)) +
			estTenue.Render(fmt.Sprintf("   (= sumar · - restar · mín %s)", formatoNum(m.piso()))) + "\n")
	} else {
		b.WriteString("\n")
	}
	b.WriteString(estTenue.Render("↑↓ scrollear · p provincia · r dólar · esc volver · q salir") + "\n")
	return strings.TrimRight(b.String(), "\n")
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

// parsearCampo saca un número de un JSON que puede mezclar tipos
// (dolarapi trae "nombre"/"casa"/"fecha..." como strings y el resto números).
func parsearCampo(b []byte, campo string) (float64, error) {
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
	return 0, fmt.Errorf("campo %q ausente", campo)
}

// actualizarFilas reconstruye la lista visible (grupo → subgrupo → juego → ítem).
func (m *modelo) actualizarFilas() {
	grupos := configuracion.BuscarArbol(m.cfg.Servicios, m.input.Value())
	m.filas = m.filas[:0]
	for _, g := range grupos {
		if g.Total == 1 && len(g.Subgrupos) == 1 && len(g.Subgrupos[0].Juegos) == 1 && len(g.Subgrupos[0].Juegos[0].Items) == 1 {
			it := g.Subgrupos[0].Juegos[0].Items[0]
			m.filas = append(m.filas, fila{tipo: 3, clave: g.Clave, nombre: it.Nombre, item: it})
			continue
		}
		m.filas = append(m.filas, fila{tipo: 0, clave: g.Clave, nombre: g.Nombre, n: g.Total})
		if !m.expandido["g:"+g.Clave] {
			continue
		}
		for _, sg := range g.Subgrupos {
			if sg.Count == 1 && len(sg.Juegos) == 1 && len(sg.Juegos[0].Items) == 1 {
				it := sg.Juegos[0].Items[0]
				m.filas = append(m.filas, fila{tipo: 3, clave: g.Clave, sub: sg.Nombre, nombre: it.Nombre, item: it})
				continue
			}
			m.filas = append(m.filas, fila{tipo: 1, clave: g.Clave, sub: sg.Nombre, nombre: sg.Nombre, n: sg.Count})
			if !m.expandido["s:"+g.Clave+"|"+sg.Nombre] {
				continue
			}
			for _, j := range sg.Juegos {
				if len(j.Items) == 1 {
					it := j.Items[0]
					m.filas = append(m.filas, fila{tipo: 3, sub: sg.Nombre, clave: g.Clave, nombre: it.Nombre, item: it})
					continue
				}
				m.filas = append(m.filas, fila{tipo: 2, clave: g.Clave, sub: j.Nombre, nombre: j.Nombre})
				for _, it := range j.Items {
					m.filas = append(m.filas, fila{tipo: 3, sub: j.Nombre, clave: g.Clave, nombre: it.Nombre, item: it})
				}
			}
		}
	}
	m.cursor = clamp(m.cursor, 0, max(0, len(m.filas)-1))
	m.ajustarScrollLista()
}

// ---- geolocalización por IP (default de provincia) ----

func detectarProvincia(ctx context.Context, provincias []string) string {
	c := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://ipwho.is/", nil)
	if err != nil {
		return ""
	}
	resp, err := c.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	var d struct {
		CountryCode string `json:"country_code"`
		Region      string `json:"region"`
		City        string `json:"city"`
	}
	if json.Unmarshal(b, &d) != nil || d.CountryCode != "AR" {
		return ""
	}
	region := d.Region
	if region == "" {
		region = d.City
	}
	return mapearProvincia(region, provincias)
}

var sinAcentos = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n")

func normalizar(s string) string {
	s = sinAcentos.Replace(strings.ToLower(s))
	s = strings.TrimSuffix(strings.TrimSpace(s), " province")
	s = strings.TrimSuffix(s, " provincia")
	return strings.TrimSpace(s)
}

func mapearProvincia(region string, provincias []string) string {
	n := normalizar(region)
	if n == "" {
		return ""
	}
	if strings.Contains(n, "capital federal") || strings.Contains(n, "federal district") ||
		(strings.Contains(n, "ciudad") && strings.Contains(n, "buenos aires")) {
		return "CABA"
	}
	if strings.Contains(n, "buenos aires") {
		return "PBA"
	}
	for _, p := range provincias {
		if normalizar(p) == n {
			return p
		}
	}
	return ""
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

// mejoresIDs devuelve el set de ids de los medios empatados en el mejor total.
func mejoresIDs(calculos []calculo) map[string]bool {
	ids := map[string]bool{}
	if len(calculos) == 0 {
		return ids
	}
	mejor := calculos[0].desglose.TotalARS
	limite := mejor + math.Max(0.5, mejor*0.001)
	for _, c := range calculos {
		if c.desglose.TotalARS <= limite {
			ids[c.medio.ID] = true
		}
	}
	return ids
}

// mejoresNombres lista los medios con el MEJOR total (incluye empates).
func mejoresNombres(calculos []calculo, max int) string {
	if len(calculos) == 0 {
		return ""
	}
	mejor := calculos[0].desglose.TotalARS
	limite := mejor + math.Max(0.5, mejor*0.001)
	var nombres []string
	for _, c := range calculos {
		if c.desglose.TotalARS <= limite {
			nombres = append(nombres, nombreMedio(c.medio))
		}
	}
	if len(nombres) > max {
		return strings.Join(nombres[:max], " · ") + fmt.Sprintf(" +%d más", len(nombres)-max)
	}
	return strings.Join(nombres, " · ")
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
