// Comando agente: mantiene la config fresca. Detecta fuentes que cambiaron,
// pide a DeepSeek una propuesta y escribe un reporte. No aplica nada solo.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/agente"
	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/llaves"
)

const maxContenido = 20000

func main() {
	fuentesPath := flag.String("fuentes", "config/fuentes.json", "archivo de fuentes a monitorear")
	estadoPath := flag.String("estado", "agente/estado.json", "archivo de estado (hashes)")
	outDir := flag.String("out", "agente", "directorio de salida (reporte.md, propuesta.json)")
	flag.Parse()

	if err := correr(*fuentesPath, *estadoPath, *outDir); err != nil {
		log.Fatalf("agente: %v", err)
	}
}

func correr(fuentesPath, estadoPath, outDir string) error {
	fuentes, err := leerFuentes(fuentesPath)
	if err != nil {
		return err
	}
	estado, err := leerEstado(estadoPath)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cliente := &http.Client{Timeout: 30 * time.Second}
	bajar := func(url string) ([]byte, error) { return fetch(ctx, cliente, url) }

	cambios, nuevos, fallaron := agente.DetectarCambios(fuentes.Fuentes, estado.Hashes, bajar)

	// El estado se escribe SIEMPRE: sirve de delta para la próxima y de latido
	// (evita que GitHub deshabilite el cron a los 60 días sin actividad).
	if err := escribirEstado(estadoPath, agente.NuevoEstado(nuevos)); err != nil {
		return err
	}
	log.Printf("fuentes=%d cambios=%d fallaron=%d", len(fuentes.Fuentes), len(cambios), len(fallaron))

	if len(cambios) == 0 {
		log.Print("sin cambios; nada que proponer")
		return nil
	}

	apiKey := resolverKey()
	llmActivo := apiKey != ""

	var prop *agente.Propuesta
	if llmActivo {
		llm := agente.NuevoLLM("https://api.deepseek.com", apiKey, "deepseek-chat")
		prop, err = llm.PreguntarPropuesta(ctx, sistemaPrompt(), promptUsuario(cambios))
		if err != nil {
			log.Printf("LLM falló: %v", err)
			llmActivo = false
		}
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	reporte := agente.Redactar(cambios, prop, fallaron, llmActivo)
	if err := os.WriteFile(filepath.Join(outDir, "reporte.md"), []byte(reporte), 0o644); err != nil {
		return err
	}
	if prop != nil {
		b, _ := json.MarshalIndent(prop, "", "  ")
		if err := os.WriteFile(filepath.Join(outDir, "propuesta.json"), append(b, '\n'), 0o644); err != nil {
			return err
		}
	}
	log.Printf("reporte escrito en %s/reporte.md", outDir)
	return nil
}

func leerFuentes(path string) (agente.Fuentes, error) {
	var f agente.Fuentes
	b, err := os.ReadFile(path)
	if err != nil {
		return f, err
	}
	return f, json.Unmarshal(b, &f)
}

func leerEstado(path string) (agente.Estado, error) {
	e := agente.Estado{Hashes: map[string]string{}}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return e, nil
	}
	if err != nil {
		return e, err
	}
	if err := json.Unmarshal(b, &e); err != nil {
		return e, err
	}
	if e.Hashes == nil {
		e.Hashes = map[string]string{}
	}
	return e, nil
}

func escribirEstado(path string, e agente.Estado) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(e, "", "  ")
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func fetch(ctx context.Context, c *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mancommunis-agente/0.1 (+https://github.com/mancommunis)")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 2<<20))
}

func sistemaPrompt() string {
	return "Sos un asistente que mantiene la config fiscal (JSON) de una calculadora de precios " +
		"digitales de Argentina (juegos, suscripciones, IA). Recibís el contenido de fuentes que " +
		"cambiaron. Proponé SOLO cambios respaldados por esas fuentes. " +
		"Devolvé JSON con esta forma exacta: " +
		`{"resumen": string, "propuestas": [{"archivo": string, "campo": string, "valor_actual": string, ` +
		`"valor_nuevo": string, "fuente_url": string, "confianza": number, "razon": string}]}. ` +
		"confianza va de 0 a 1. Si no hay nada relevante, devolvé {\"propuestas\": []}. " +
		"Nunca inventes alícuotas sin fuente."
}

func promptUsuario(cambios []agente.Cambio) string {
	var b strings.Builder
	for _, c := range cambios {
		b.WriteString("### Fuente: " + c.Fuente.ID + " (" + c.Fuente.URL + ")\n")
		if c.Fuente.Informa != "" {
			b.WriteString("Informa a: " + c.Fuente.Informa + "\n")
		}
		b.WriteString("Contenido:\n")
		b.WriteString(truncar(c.Contenido, maxContenido))
		b.WriteString("\n\n")
	}
	return b.String()
}

func truncar(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n…[truncado]"
}

// resolverKey busca la API key: primero el env (CI/CI secrets), luego el
// keychain del SO (desarrollo local). Nunca se hardcodea.
func resolverKey() string {
	if k := os.Getenv("DEEPSEEK_API_KEY"); k != "" {
		return k
	}
	if k, err := llaves.Get("deepseek"); err == nil {
		return k
	}
	return ""
}
