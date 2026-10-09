//! Motor fiscal puro para precios digitales en Argentina.
//!
//! No hace I/O: recibe todo resuelto (parámetros fiscales y cotizaciones) y
//! devuelve el desglose. "No sabe nada de la pantalla", ni del host que lo llama
//! (nativo, WASM en el browser, o WASM embebido en Go vía wazero).
//!
//! Modelo: los impuestos se suman **aditivos sobre la base** (21% + 30% + 2% ≈ +53%),
//! que es como se leen los totales de referencia en Argentina. Ver `config/tax-config.json`.

use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

pub type Resultado = Result<Desglose, Error>;

/// Moneda en la que la tienda muestra el precio.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Moneda {
    Ars,
    Usd,
}

/// Régimen impositivo. Determina IVA y percepción de Ganancias.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Regimen {
    Gaming,
    ServiciosDigitales,
}

/// Tipo de medio de pago. Determina qué impuestos aplican.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum TipoPago {
    /// Tarjeta/billetera emitida en Argentina. Suma IVA + IIBB + Ganancias.
    TarjetaLocal,
    /// Pago con dólares propios (MEP / caja de ahorro). Suma IVA + IIBB; evita Ganancias.
    DolaresPropios,
    /// Tarjeta emitida en el exterior. Sin percepciones locales.
    TarjetaGlobal,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
pub struct Precio {
    pub moneda: Moneda,
    pub monto: f64,
}

/// Parámetros fiscales ya resueltos para un (régimen, provincia).
#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
pub struct ParametrosFiscales {
    pub iva: f64,
    pub ganancias: f64,
    pub iibb: f64,
    #[serde(default)]
    pub impuesto_pais: f64,
}

/// Un medio de pago concreto (ej.: "ARQ local", "tarjeta del banco", "dólares propios").
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MedioPago {
    pub nombre: String,
    pub tipo: TipoPago,
    /// Fuente de cotización a usar: "oficial", "mep", "ccl", "cripto", "arq", ...
    pub fx: String,
    /// Comisión/recargo propio de la billetera (fracción, ej. 0.02 = 2%).
    #[serde(default)]
    pub recargo_extra: f64,
}

/// Cotizaciones disponibles, por fuente (nombre -> ARS por USD).
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(transparent)]
pub struct Cotizaciones(BTreeMap<String, f64>);

impl Cotizaciones {
    pub fn nueva() -> Self {
        Self(BTreeMap::new())
    }

    pub fn con(mut self, fuente: impl Into<String>, valor: f64) -> Self {
        self.0.insert(fuente.into(), valor);
        self
    }

    pub fn insertar(&mut self, fuente: impl Into<String>, valor: f64) {
        self.0.insert(fuente.into(), valor);
    }

    pub fn get(&self, fuente: &str) -> Option<f64> {
        self.0.get(fuente).copied()
    }
}

/// Entrada de cálculo.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Entrada {
    pub precio: Precio,
    pub regimen: Regimen,
    pub parametros: ParametrosFiscales,
    pub medio: MedioPago,
    #[serde(default)]
    pub cotizaciones: Cotizaciones,
}

/// Resultado del cálculo, en pesos.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Desglose {
    pub regimen: Regimen,
    pub moneda_precio: Moneda,
    pub monto_precio: f64,
    pub fx_nombre: Option<String>,
    pub fx_valor: Option<f64>,
    pub base_ars: f64,
    pub iva: f64,
    pub iibb: f64,
    pub ganancias: f64,
    pub impuesto_pais: f64,
    pub recargo_extra: f64,
    pub total_ars: f64,
    /// Recargo efectivo sobre la base (`total/base − 1`).
    pub tasa_efectiva: f64,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case", tag = "tipo")]
pub enum Error {
    FaltaCotizacion { fuente: String },
    MontoInvalido,
    RegimenDesconocido { regimen: String },
    ProvinciaDesconocida { provincia: String },
}

impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Error::FaltaCotizacion { fuente } => write!(f, "falta cotización para '{fuente}'"),
            Error::MontoInvalido => write!(f, "el monto del precio es inválido"),
            Error::RegimenDesconocido { regimen } => write!(f, "régimen desconocido: {regimen}"),
            Error::ProvinciaDesconocida { provincia } => {
                write!(f, "provincia sin alícuota de IIBB: {provincia}")
            }
        }
    }
}

impl std::error::Error for Error {}

/// Config fiscal (config-as-code). Ignora claves extra como `_notas` o `vigente_*`.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConfigFiscal {
    #[serde(default)]
    pub impuesto_pais: f64,
    pub regimenes: BTreeMap<Regimen, RegimenParams>,
    pub iibb: BTreeMap<String, f64>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
pub struct RegimenParams {
    pub iva: f64,
    pub ganancias: f64,
}

impl ConfigFiscal {
    /// Resuelve los parámetros fiscales para un régimen y una provincia.
    pub fn parametros(
        &self,
        regimen: Regimen,
        provincia: &str,
    ) -> Result<ParametrosFiscales, Error> {
        let r = self
            .regimenes
            .get(&regimen)
            .ok_or_else(|| Error::RegimenDesconocido {
                regimen: format!("{regimen:?}"),
            })?;
        let iibb = *self
            .iibb
            .get(provincia)
            .ok_or_else(|| Error::ProvinciaDesconocida {
                provincia: provincia.to_string(),
            })?;
        Ok(ParametrosFiscales {
            iva: r.iva,
            ganancias: r.ganancias,
            iibb,
            impuesto_pais: self.impuesto_pais,
        })
    }
}

/// Aplicabilidad de cada impuesto según el tipo de pago.
#[derive(Debug, Clone, Copy)]
struct Aplica {
    iva: bool,
    iibb: bool,
    ganancias: bool,
}

fn aplicacion(tipo: TipoPago) -> Aplica {
    match tipo {
        // Operación del exterior: sin percepciones locales.
        TipoPago::TarjetaGlobal => Aplica {
            iva: false,
            iibb: false,
            ganancias: false,
        },
        // Dólares propios: siguen IVA e IIBB; se evita la percepción de Ganancias.
        TipoPago::DolaresPropios => Aplica {
            iva: true,
            iibb: true,
            ganancias: false,
        },
        // Tarjeta local: todo.
        TipoPago::TarjetaLocal => Aplica {
            iva: true,
            iibb: true,
            ganancias: true,
        },
    }
}

/// Calcula el precio final en pesos.
pub fn calcular(entrada: &Entrada) -> Resultado {
    if !entrada.precio.monto.is_finite() || entrada.precio.monto < 0.0 {
        return Err(Error::MontoInvalido);
    }

    let (base_ars, fx_nombre, fx_valor) = match entrada.precio.moneda {
        Moneda::Ars => (entrada.precio.monto, None, None),
        Moneda::Usd => {
            let valor = entrada.cotizaciones.get(&entrada.medio.fx).ok_or_else(|| {
                Error::FaltaCotizacion {
                    fuente: entrada.medio.fx.clone(),
                }
            })?;
            (
                entrada.precio.monto * valor,
                Some(entrada.medio.fx.clone()),
                Some(valor),
            )
        }
    };

    let ap = aplicacion(entrada.medio.tipo);
    let p = &entrada.parametros;

    let iva = if ap.iva { base_ars * p.iva } else { 0.0 };
    let iibb = if ap.iibb { base_ars * p.iibb } else { 0.0 };
    let ganancias = if ap.ganancias {
        base_ars * p.ganancias
    } else {
        0.0
    };
    let impuesto_pais = base_ars * p.impuesto_pais;
    let recargo_extra = base_ars * entrada.medio.recargo_extra;

    let total_ars = base_ars + iva + iibb + ganancias + impuesto_pais + recargo_extra;
    let tasa_efectiva = if base_ars > 0.0 {
        total_ars / base_ars - 1.0
    } else {
        0.0
    };

    Ok(Desglose {
        regimen: entrada.regimen,
        moneda_precio: entrada.precio.moneda,
        monto_precio: entrada.precio.monto,
        fx_nombre,
        fx_valor,
        base_ars,
        iva,
        iibb,
        ganancias,
        impuesto_pais,
        recargo_extra,
        total_ars,
        tasa_efectiva,
    })
}

/// Interfaz portable (WASM/Go/JS): JSON entra, JSON sale.
pub fn calcular_json(input: &str) -> Result<String, String> {
    let entrada: Entrada = serde_json::from_str(input).map_err(|e| e.to_string())?;
    let desglose = calcular(&entrada).map_err(|e| e.to_string())?;
    serde_json::to_string(&desglose).map_err(|e| e.to_string())
}

/// ABI plano para hosts que cargan WASM sin wasm-bindgen (ej.: Go con wazero).
/// Convención: `cfg_calcular_json(ptr, len)` devuelve un buffer `[u32 LE len][bytes]`,
/// que el host lee y luego libera con `cfg_free(ptr, 4 + len)`.
#[cfg(target_os = "wasi")]
pub mod abi {
    use std::alloc::{alloc, dealloc, Layout};

    fn layout(len: usize) -> Layout {
        Layout::from_size_align(len, 1).expect("layout válido")
    }

    #[no_mangle]
    pub extern "C" fn cfg_alloc(len: usize) -> *mut u8 {
        if len == 0 {
            return std::ptr::null_mut();
        }
        unsafe { alloc(layout(len)) }
    }

    #[no_mangle]
    pub extern "C" fn cfg_free(ptr: *mut u8, len: usize) {
        if ptr.is_null() || len == 0 {
            return;
        }
        unsafe { dealloc(ptr, layout(len)) }
    }

    fn empaquetar(bytes: &[u8]) -> *mut u8 {
        let total = 4 + bytes.len();
        let buf = unsafe { alloc(layout(total)) };
        if buf.is_null() {
            return buf;
        }
        let len_le = (bytes.len() as u32).to_le_bytes();
        unsafe {
            std::ptr::copy_nonoverlapping(len_le.as_ptr(), buf, 4);
            std::ptr::copy_nonoverlapping(bytes.as_ptr(), buf.add(4), bytes.len());
        }
        buf
    }

    #[no_mangle]
    pub extern "C" fn cfg_calcular_json(ptr: *const u8, len: usize) -> *mut u8 {
        let input = if ptr.is_null() || len == 0 {
            String::new()
        } else {
            let slice = unsafe { std::slice::from_raw_parts(ptr, len) };
            String::from_utf8_lossy(slice).into_owned()
        };
        let out = match crate::calcular_json(&input) {
            Ok(s) => s,
            Err(e) => serde_json::json!({ "error": e }).to_string(),
        };
        empaquetar(out.as_bytes())
    }
}

#[cfg(all(target_arch = "wasm32", target_os = "unknown"))]
mod wasm {
    use wasm_bindgen::prelude::*;

    /// Export para el browser (wasm-bindgen).
    #[wasm_bindgen]
    pub fn calcular_json(input: &str) -> String {
        match crate::calcular_json(input) {
            Ok(s) => s,
            Err(e) => serde_json::json!({ "error": e }).to_string(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn aprox(a: f64, b: f64) {
        assert!((a - b).abs() < 1e-9, "esperado {b}, obtenido {a}");
    }

    fn parametros(iva: f64, ganancias: f64, iibb: f64) -> ParametrosFiscales {
        ParametrosFiscales {
            iva,
            ganancias,
            iibb,
            impuesto_pais: 0.0,
        }
    }

    fn medio(tipo: TipoPago, fx: &str) -> MedioPago {
        MedioPago {
            nombre: "test".into(),
            tipo,
            fx: fx.into(),
            recargo_extra: 0.0,
        }
    }

    #[test]
    fn gaming_ars_tarjeta_local_suma_23() {
        let e = Entrada {
            precio: Precio {
                moneda: Moneda::Ars,
                monto: 1000.0,
            },
            regimen: Regimen::Gaming,
            parametros: parametros(0.21, 0.0, 0.02),
            medio: medio(TipoPago::TarjetaLocal, "oficial"),
            cotizaciones: Cotizaciones::nueva(),
        };
        let d = calcular(&e).unwrap();
        aprox(d.base_ars, 1000.0);
        aprox(d.iva, 210.0);
        aprox(d.iibb, 20.0);
        aprox(d.ganancias, 0.0);
        aprox(d.total_ars, 1230.0);
        aprox(d.tasa_efectiva, 0.23);
    }

    #[test]
    fn servicios_usd_tarjeta_local_suma_53() {
        let e = Entrada {
            precio: Precio {
                moneda: Moneda::Usd,
                monto: 10.0,
            },
            regimen: Regimen::ServiciosDigitales,
            parametros: parametros(0.21, 0.30, 0.02),
            medio: medio(TipoPago::TarjetaLocal, "oficial"),
            cotizaciones: Cotizaciones::nueva().con("oficial", 1000.0),
        };
        let d = calcular(&e).unwrap();
        aprox(d.base_ars, 10_000.0);
        aprox(d.iva, 2_100.0);
        aprox(d.iibb, 200.0);
        aprox(d.ganancias, 3_000.0);
        aprox(d.total_ars, 15_300.0);
        aprox(d.tasa_efectiva, 0.53);
        assert_eq!(d.fx_nombre.as_deref(), Some("oficial"));
    }

    #[test]
    fn gaming_usd_tarjeta_global_sin_percepciones() {
        let e = Entrada {
            precio: Precio {
                moneda: Moneda::Usd,
                monto: 10.0,
            },
            regimen: Regimen::Gaming,
            parametros: parametros(0.21, 0.0, 0.02),
            medio: medio(TipoPago::TarjetaGlobal, "arq"),
            cotizaciones: Cotizaciones::nueva().con("arq", 1000.0),
        };
        let d = calcular(&e).unwrap();
        aprox(d.base_ars, 10_000.0);
        aprox(d.iva, 0.0);
        aprox(d.iibb, 0.0);
        aprox(d.ganancias, 0.0);
        aprox(d.total_ars, 10_000.0);
        aprox(d.tasa_efectiva, 0.0);
    }

    #[test]
    fn servicios_dolares_propios_evita_ganancias() {
        let e = Entrada {
            precio: Precio {
                moneda: Moneda::Usd,
                monto: 10.0,
            },
            regimen: Regimen::ServiciosDigitales,
            parametros: parametros(0.21, 0.30, 0.02),
            medio: medio(TipoPago::DolaresPropios, "mep"),
            cotizaciones: Cotizaciones::nueva().con("mep", 1000.0),
        };
        let d = calcular(&e).unwrap();
        aprox(d.base_ars, 10_000.0);
        aprox(d.ganancias, 0.0);
        aprox(d.total_ars, 12_300.0);
        aprox(d.tasa_efectiva, 0.23);
    }

    #[test]
    fn recargo_de_billetera_se_suma() {
        let mut m = medio(TipoPago::TarjetaLocal, "oficial");
        m.recargo_extra = 0.05;
        let e = Entrada {
            precio: Precio {
                moneda: Moneda::Ars,
                monto: 1000.0,
            },
            regimen: Regimen::Gaming,
            parametros: parametros(0.21, 0.0, 0.02),
            medio: m,
            cotizaciones: Cotizaciones::nueva(),
        };
        let d = calcular(&e).unwrap();
        aprox(d.recargo_extra, 50.0);
        aprox(d.total_ars, 1280.0);
    }

    #[test]
    fn usd_sin_cotizacion_falla() {
        let e = Entrada {
            precio: Precio {
                moneda: Moneda::Usd,
                monto: 10.0,
            },
            regimen: Regimen::Gaming,
            parametros: parametros(0.21, 0.0, 0.02),
            medio: medio(TipoPago::TarjetaLocal, "oficial"),
            cotizaciones: Cotizaciones::nueva(),
        };
        assert_eq!(
            calcular(&e).unwrap_err(),
            Error::FaltaCotizacion {
                fuente: "oficial".into()
            }
        );
    }

    #[test]
    fn config_resuelve_parametros() {
        let cfg: ConfigFiscal = serde_json::from_str(
            r#"{
                "impuesto_pais": 0.0,
                "regimenes": {
                    "gaming": { "iva": 0.21, "ganancias": 0.0 },
                    "servicios_digitales": { "iva": 0.21, "ganancias": 0.30 }
                },
                "iibb": { "CABA": 0.02, "PBA": 0.02 },
                "_notas": { "ignorame": true }
            }"#,
        )
        .unwrap();

        let p = cfg.parametros(Regimen::Gaming, "CABA").unwrap();
        aprox(p.iva, 0.21);
        aprox(p.ganancias, 0.0);
        aprox(p.iibb, 0.02);

        let p2 = cfg.parametros(Regimen::ServiciosDigitales, "PBA").unwrap();
        aprox(p2.ganancias, 0.30);

        assert!(cfg.parametros(Regimen::Gaming, "Marte").is_err());
    }

    #[test]
    fn json_roundtrip() {
        let input = r#"{
            "precio": { "moneda": "usd", "monto": 10 },
            "regimen": "servicios_digitales",
            "parametros": { "iva": 0.21, "ganancias": 0.30, "iibb": 0.02, "impuesto_pais": 0.0 },
            "medio": { "nombre": "oficial", "tipo": "tarjeta_local", "fx": "oficial", "recargo_extra": 0.0 },
            "cotizaciones": { "oficial": 1000.0 }
        }"#;
        let out = calcular_json(input).unwrap();
        let d: Desglose = serde_json::from_str(&out).unwrap();
        aprox(d.total_ars, 15_300.0);
    }
}
