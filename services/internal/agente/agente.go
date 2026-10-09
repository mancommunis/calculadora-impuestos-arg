// Package agente mantiene la config "fresca": detecta fuentes que cambiaron y
// le pide a un LLM una propuesta de actualización. NUNCA la aplica solo —
// el resultado es un PR para revisión humana.
package agente

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)

// Fuente es un origen a monitorear (una URL que informa la config).
type Fuente struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	Informa string `json:"informa"`
	Notas   string `json:"notas,omitempty"`
}

// Fuentes es el archivo config/fuentes.json.
type Fuentes struct {
	Fuentes []Fuente `json:"fuentes"`
}

// Estado persiste el hash de cada fuente entre corridas (permite delta-only).
type Estado struct {
	UltimaCorrida string            `json:"ultima_corrida"`
	Hashes        map[string]string `json:"hashes"`
}

// Cambio es una fuente cuyo contenido difiere del estado previo.
type Cambio struct {
	Fuente    Fuente
	Contenido string
}

// CambioPropuesto es una actualización sugerida por el LLM, con trazabilidad.
type CambioPropuesto struct {
	Archivo     string  `json:"archivo"`
	Campo       string  `json:"campo"`
	ValorActual string  `json:"valor_actual,omitempty"`
	ValorNuevo  string  `json:"valor_nuevo"`
	FuenteURL   string  `json:"fuente_url"`
	Confianza   float64 `json:"confianza"`
	Razon       string  `json:"razon"`
}

// Propuesta es la salida estructurada del LLM.
type Propuesta struct {
	Resumen    string            `json:"resumen,omitempty"`
	Propuestas []CambioPropuesto `json:"propuestas"`
}

// Hash devuelve el sha256 hex del contenido.
func Hash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// DetectarCambios compara el hash de cada fuente contra el estado previo.
// `bajar` inyecta el fetch (real en prod, fake en tests). Una fuente que falla
// se omite y conserva su hash anterior.
func DetectarCambios(
	fuentes []Fuente,
	hashes map[string]string,
	bajar func(url string) ([]byte, error),
) (cambios []Cambio, nuevos map[string]string, fallaron []string) {
	nuevos = map[string]string{}
	for _, f := range fuentes {
		contenido, err := bajar(f.URL)
		if err != nil {
			fallaron = append(fallaron, f.ID)
			if h, ok := hashes[f.URL]; ok {
				nuevos[f.URL] = h
			}
			continue
		}
		h := Hash(contenido)
		nuevos[f.URL] = h
		if hashes[f.URL] != h {
			cambios = append(cambios, Cambio{Fuente: f, Contenido: string(contenido)})
		}
	}
	sort.Slice(cambios, func(i, j int) bool { return cambios[i].Fuente.ID < cambios[j].Fuente.ID })
	sort.Strings(fallaron)
	return cambios, nuevos, fallaron
}

// NuevoEstado arma el estado con timestamp UTC.
func NuevoEstado(hashes map[string]string) Estado {
	return Estado{UltimaCorrida: time.Now().UTC().Format(time.RFC3339), Hashes: hashes}
}

// Redactar arma el reporte en markdown que viaja en el PR.
func Redactar(cambios []Cambio, prop *Propuesta, fallaron []string, llmActivo bool) string {
	s := "# Config refresh — propuesta del agente\n\n"
	s += fmt.Sprintf("Generado: %s\n\n", time.Now().UTC().Format(time.RFC3339))

	if len(cambios) == 0 {
		s += "Sin cambios en las fuentes monitoreadas. Nada que proponer.\n"
	} else {
		s += "## Fuentes que cambiaron\n\n"
		for _, c := range cambios {
			s += fmt.Sprintf("- `%s` — %s\n", c.Fuente.ID, c.Fuente.URL)
		}
		s += "\n"
	}

	if len(fallaron) > 0 {
		s += "## Fuentes que no se pudieron leer\n\n"
		for _, id := range fallaron {
			s += fmt.Sprintf("- `%s`\n", id)
		}
		s += "\n"
	}

	if !llmActivo {
		s += "> LLM desactivado (falta `DEEPSEEK_API_KEY`). No se generaron propuestas.\n"
		return s
	}

	if prop == nil {
		return s
	}

	if prop.Resumen != "" {
		s += "## Resumen\n\n" + prop.Resumen + "\n\n"
	}

	s += "## Propuestas\n\n"
	if len(prop.Propuestas) == 0 {
		s += "El agente no encontró cambios relevantes para la config.\n"
		return s
	}
	for _, p := range prop.Propuestas {
		s += fmt.Sprintf("### %s · %s\n\n", p.Archivo, p.Campo)
		s += fmt.Sprintf("- **Ahora:** %s\n", orDash(p.ValorActual))
		s += fmt.Sprintf("- **Propuesto:** %s\n", p.ValorNuevo)
		s += fmt.Sprintf("- **Fuente:** %s\n", p.FuenteURL)
		s += fmt.Sprintf("- **Confianza:** %.0f%%\n", p.Confianza*100)
		s += fmt.Sprintf("- **Razón:** %s\n\n", p.Razon)
	}
	s += "\n> Revisión humana obligatoria. El agente no aplica cambios solo.\n"
	return s
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
