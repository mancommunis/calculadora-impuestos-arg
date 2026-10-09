package agente

import (
	"errors"
	"strings"
	"testing"
)

func TestHashEstable(t *testing.T) {
	if Hash([]byte("hola")) != Hash([]byte("hola")) {
		t.Fatal("el hash debería ser estable")
	}
	if Hash([]byte("hola")) == Hash([]byte("hola!")) {
		t.Fatal("contenidos distintos no deberían colisionar")
	}
}

func TestDetectarCambios(t *testing.T) {
	fuentes := []Fuente{
		{ID: "a", URL: "http://a"},
		{ID: "b", URL: "http://b"},
		{ID: "c", URL: "http://c"},
	}
	contenido := map[string]string{"http://a": "contenido-a", "http://b": "nuevo-b", "http://c": "x"}
	bajar := func(url string) ([]byte, error) {
		if url == "http://c" {
			return nil, errors.New("boom")
		}
		return []byte(contenido[url]), nil
	}
	previos := map[string]string{
		"http://a": Hash([]byte("contenido-a")), // sin cambios
		"http://b": Hash([]byte("viejo-b")),     // cambió
	}

	cambios, nuevos, fallaron := DetectarCambios(fuentes, previos, bajar)

	if len(cambios) != 1 || cambios[0].Fuente.ID != "b" {
		t.Fatalf("esperaba 1 cambio (b), obtuve %+v", cambios)
	}
	if len(fallaron) != 1 || fallaron[0] != "c" {
		t.Fatalf("esperaba que fallara c, obtuve %+v", fallaron)
	}
	if nuevos["http://a"] != Hash([]byte("contenido-a")) {
		t.Fatal("hash de 'a' debería conservarse")
	}
	if nuevos["http://b"] != Hash([]byte("nuevo-b")) {
		t.Fatal("hash de 'b' debería actualizarse")
	}
}

func TestRedactarSinCambios(t *testing.T) {
	out := Redactar(nil, nil, nil, true)
	if !strings.Contains(out, "Sin cambios") {
		t.Fatalf("reporte inesperado: %s", out)
	}
}

func TestRedactarConPropuesta(t *testing.T) {
	prop := &Propuesta{
		Resumen: "cambió IVA",
		Propuestas: []CambioPropuesto{
			{Archivo: "tax-config.json", Campo: "regimenes.gaming.iva", ValorNuevo: "0.21", FuenteURL: "http://a", Confianza: 0.9, Razon: "fuente oficial"},
		},
	}
	out := Redactar([]Cambio{{Fuente: Fuente{ID: "a", URL: "http://a"}}}, prop, nil, true)
	for _, esperado := range []string{"tax-config.json", "regimenes.gaming.iva", "http://a", "90%"} {
		if !strings.Contains(out, esperado) {
			t.Fatalf("falta %q en el reporte:\n%s", esperado, out)
		}
	}
}
