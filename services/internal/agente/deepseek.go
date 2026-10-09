package agente

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LLM es un cliente chat-completions compatible con OpenAI (DeepSeek, etc.).
type LLM struct {
	BaseURL string
	APIKey  string
	Modelo  string
	Cliente *http.Client
}

// NuevoLLM arma el cliente con timeouts sanos.
func NuevoLLM(baseURL, apiKey, modelo string) LLM {
	return LLM{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Modelo:  modelo,
		Cliente: &http.Client{Timeout: 60 * time.Second},
	}
}

type mensaje struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Preguntar manda system+user y devuelve el contenido crudo de la respuesta
// (se pide salida JSON con response_format).
func (l LLM) Preguntar(ctx context.Context, system, user string) (string, error) {
	cuerpo := map[string]any{
		"model":       l.Modelo,
		"temperature": 0,
		"messages": []mensaje{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		"response_format": map[string]string{"type": "json_object"},
	}
	raw, _ := json.Marshal(cuerpo)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.APIKey)

	resp, err := l.Cliente.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("LLM %d: %s", resp.StatusCode, string(data))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("respuesta LLM inválida: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("respuesta LLM sin choices")
	}
	return out.Choices[0].Message.Content, nil
}

// PreguntarPropuesta pide y parsea la propuesta estructurada.
func (l LLM) PreguntarPropuesta(ctx context.Context, system, user string) (*Propuesta, error) {
	contenido, err := l.Preguntar(ctx, system, user)
	if err != nil {
		return nil, err
	}
	var p Propuesta
	if err := json.Unmarshal([]byte(contenido), &p); err != nil {
		return nil, fmt.Errorf("la propuesta no es JSON válido: %w", err)
	}
	return &p, nil
}
