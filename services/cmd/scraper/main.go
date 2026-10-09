// Comando scraper: actualiza los precios del dataset/catalogo.json.
//
// Steam se refresca por API; Battle.net se intenta y hoy reporta el bloqueo
// anti-bot (401), dejando el precio manual que ya tenga el catálogo.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/configuracion"
	"github.com/mancommunis/calculadora-impuestos-arg/services/internal/tiendas"
)

func main() {
	datasetPath := flag.String("dataset", "dataset/catalogo.json", "ruta del catálogo")
	cc := flag.String("cc", "ar", "país para la cotización de Steam")
	flag.Parse()

	if err := correr(*datasetPath, *cc); err != nil {
		log.Fatalf("scraper: %v", err)
	}
}

func correr(datasetPath, cc string) error {
	cat, err := configuracion.CargarCatalogo(datasetPath)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cliente := tiendas.Cliente()

	var actualizados, manuales, fallaron int
	for i := range cat.Productos {
		p := &cat.Productos[i]
		switch p.Tienda {
		case "steam":
			if p.AppID == 0 {
				continue
			}
			pt, err := tiendas.Steam(ctx, cliente, "", p.AppID, cc)
			if err != nil {
				log.Printf("steam %s (%d): %v", p.ID, p.AppID, err)
				fallaron++
				continue
			}
			monto := pt.Monto
			moneda := tiendas.NormalizaMoneda(pt.Moneda)
			p.Precio = &monto
			p.Moneda = moneda
			actualizados++
			log.Printf("steam %s: %s %.2f (desc %d%%)", p.ID, moneda, monto, pt.Descuento)

		case "battle_net":
			if _, err := tiendas.BattleNet(ctx, cliente, "https://us.shop.battle.net/en-us/product/"+p.ID); err != nil {
				log.Printf("battle_net %s: %v (queda precio manual)", p.ID, err)
			}
			manuales++

		default:
			if p.Manual {
				manuales++
			}
		}
	}

	if err := configuracion.GuardarCatalogo(datasetPath, cat); err != nil {
		return err
	}
	log.Printf("dataset escrito: %s (actualizados=%d manuales=%d fallaron=%d)", datasetPath, actualizados, manuales, fallaron)
	fmt.Printf("ok: %d actualizados, %d manuales, %d fallaron\n", actualizados, manuales, fallaron)
	return nil
}
