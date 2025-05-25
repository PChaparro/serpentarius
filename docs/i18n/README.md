# Serpentarius

<div align="center">
  <img src="../assets/logo.png" alt="Serpentarius Logo" width="200px" height="200px" />
</div>

[![Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Reference](https://pkg.go.dev/badge/github.com/PChaparro/serpentarius.svg)](https://pkg.go.dev/github.com/PChaparro/serpentarius)
[![Go Report Card](https://goreportcard.com/badge/github.com/PChaparro/serpentarius)](https://goreportcard.com/report/github.com/PChaparro/serpentarius)

Serpentarius (Alias "Pájaro Secretario") es un microservicio REST que genera documentos PDF a partir de plantillas HTML para tus proyectos.

## ¿Qué soluciona Serpentarius? 🤔

Generar PDFs a partir de HTML es una práctica común por la flexibilidad que ofrece (puedes crear casi cualquier diseño usando HTML y CSS). Sin embargo, integrar esta funcionalidad en cada proyecto presenta varios retos:

- **Instalación de navegadores**: Necesitas Chromium para renderizar el HTML, lo que aumenta considerablemente el tamaño de tus imágenes Docker y el consumo de recursos en tus servidores 💸.

- **Alto consumo de recursos**: La generación de PDFs es intensiva computacionalmente. Con un microservicio dedicado, puedes escalar esta funcionalidad de forma independiente 🚀.

- **Optimización con caché**: Serpentarius implementa caché con Redis para evitar generar repetidamente el mismo documento, una característica que no querrías implementar en cada uno de tus proyectos.

Serpentarius resuelve estos problemas exponiendo una API REST que puedes consultar desde cualquier proyecto o lenguaje de programación.

## ¿Qué **NO** soluciona Serpentarius? ❌

- Serpentarius **no** almacena plantillas HTML. Tu proyecto debe encargarse de esto y enviar el HTML en cada solicitud. Esto hace que Serpentarius sea agnóstico en cuanto a tecnologías y lenguajes de programación, **solo necesitas enviar HTML válido y Serpentarius lo convertirá a PDF**.
- Serpentarius **no** optimiza el HTML que recibe. Tu proyecto debe encargarse de aplicar buenas prácticas como usar imágenes con tamaños adecuados, evitar fuentes pesadas y eliminar estilos innecesarios. Esto ayuda a reducir el tamaño del PDF resultante y garantiza un mejor rendimiento. Serpentarius renderiza exactamente lo que recibe, por lo que **no** realiza modificaciones para evitar resultados inesperados.

## Instalación ⬇️

Este proyecto está diseñado para funcionar como un microservicio REST, no como una librería.

Puedes usar Docker o compilar el proyecto para ejecutarlo. En ambos casos, necesitarás:

- Almacenamiento compatible con API de S3 (para guardar documentos generados) 📂
- Servidor compatible con API de Redis (para implementar caché) ⚡
- Chromium o navegador similar (para renderizar los documentos) 🖥️

### Build 🛠️

Para compilar el proyecto necesitas [Go](https://golang.org/dl/).

Una vez clonado el proyecto, compílalo con:

```bash
go build -o serpentarius.bin cmd/http/main.go
```

Esto generará el binario `serpentarius.bin` en la raíz del proyecto, a partir del punto de entrada `cmd/http/main.go`.

Para ejecutarlo:

```bash
./serpentarius.bin
```

### Docker 🐳

Con el proyecto clonado, construye la imagen Docker:

```bash
docker build -f Containerfile -t serpentarius .
```

Y ejecuta el contenedor:

```bash
docker run -p 3000:3000 -e AWS_S3_ENDPOINT_URL=http://localhost:9000 \
  -e AWS_ACCESS_KEY_ID=value \
  -e AWS_SECRET_ACCESS_KEY=value \
  -e AWS_REGION=us-east-1 \
  -e REDIS_HOST=localhost \
  -e REDIS_PORT=6379 \
  -e REDIS_PASSWORD=dragonfly \
  -e REDIS_DB=0 \
  -e AUTH_SECRET=your_secret_key \
  -e CHROMIUM_BINARY_PATH=/usr/bin/chromium \
  -e MAX_CHROMIUM_BROWSERS=1 \
  -e MAX_CHROMIUM_TABS_PER_BROWSER=4 \
  -e MAX_IDLE_SECONDS=30 \
  -e ENVIRONMENT=production \
  serpentarius
```

### Variables de entorno 🌍

Para cualquier método de instalación, configura estas variables en un archivo `.env` o en el entorno:

| Nombre                          | Descripción                                                            | Valor para desarrollo                                                                          |
| ------------------------------- | ---------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| `AWS_S3_ENDPOINT_URL`           | URL del endpoint de S3                                                 | `http://localhost:9000`                                                                        |
| `AWS_ACCESS_KEY_ID`             | ID de la clave de acceso de AWS                                        | Debes crear un Bucket y copiar el `Access Key ID` de un usuario que tenga acceso al Bucket     |
| `AWS_SECRET_ACCESS_KEY`         | Clave de acceso secreta de AWS                                         | Debes crear un Bucket y copiar el `Secret Access Key` de un usuario que tenga acceso al Bucket |
| `AWS_REGION`                    | Región de AWS donde se encuentra el Bucket                             | Por defecto se usa el valor `us-east-1`                                                        |
| `REDIS_HOST`                    | Hostname del servidor Redis                                            | `localhost`                                                                                    |
| `REDIS_PORT`                    | Puerto del servidor Redis                                              | `6379`                                                                                         |
| `REDIS_PASSWORD`                | Contraseña del servidor Redis                                          | `dragonfly`                                                                                    |
| `REDIS_DB`                      | Base de datos de Redis a utilizar                                      | `0`                                                                                            |
| `AUTH_SECRET`                   | Clave secreta para la autenticación de usuarios                        | No se establece valor por defecto                                                              |
| `CHROMIUM_BINARY_PATH`          | Ruta al binario de Chromium                                            | `/usr/bin/chromium`                                                                            |
| `MAX_CHROMIUM_BROWSERS`         | Número máximo de navegadores Chromium concurrentes                     | `1`                                                                                            |
| `MAX_CHROMIUM_TABS_PER_BROWSER` | Número máximo de pestañas por navegador Chromium                       | `4`                                                                                            |
| `MAX_IDLE_SECONDS`              | Segundos máximos que una página puede estar inactiva antes de cerrarse | `30`                                                                                           |
| `ENVIRONMENT`                   | Entorno de ejecución (development/production)                          | `development`                                                                                  |

Los valores mostrados en la columna `Valor para desarrollo` son compatibles con el archivo `container-compose.yml` incluido en el proyecto, que configura Dragonfly (alternativa a Redis) y MinIO (alternativa a S3) para desarrollo local. Si usas tus propios servidores, ajusta estas variables según corresponda.

Para generar el secreto de autenticación, puedes usar el siguiente comando:

```bash
openssl rand -base64 64
```
