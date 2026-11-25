# Micro-Test Inventory API

Microservicio de inventario desarrollado en Go. Sirve como base de referencia para construir servicios HTTP con persistencia, validación, pruebas automatizadas y ejecución reproducible.

## Funcionalidades

- API REST para crear, listar, consultar, actualizar y eliminar artículos.
- PostgreSQL con creación automática de la tabla al iniciar el servicio.
- Validación de solicitudes JSON y errores HTTP consistentes.
- Chequeos de salud diferenciados: liveness y readiness.
- Apagado graceful y límites de tiempo para las conexiones HTTP.
- Arquitectura hexagonal: aplicación y dominio aislados de los adaptadores HTTP y PostgreSQL.
- Pruebas unitarias del servicio con un repositorio en memoria y del adaptador HTTP con casos simulados.
- Docker multi-stage y entorno local con Docker Compose.
- OpenAPI/Swagger en `/swagger/index.html`.
- CI con compilación, pruebas con `-race`, formato, `go vet` y build de Docker.

## Requisitos

- Go 1.23.3.
- Docker y Docker Compose para iniciar la API junto con PostgreSQL.

## Inicio rápido

Desde la raíz del repositorio:

```bash
docker compose up --build -d
```

Verifica el estado y los logs:

```bash
docker compose ps
docker compose logs -f micro-test
```

La API queda disponible en `http://localhost:8089` y Swagger en `http://localhost:8089/swagger/index.html`.
PostgreSQL conserva los datos en el volumen `postgres_data`; `docker compose down` detiene los servicios sin borrar ese volumen.

Para detener los servicios:

```bash
docker compose down
```

> En desarrollo, Compose usa credenciales locales de ejemplo. Cámbialas y gestiona los secretos de forma segura antes de desplegar el servicio fuera de un entorno local.

## API

Cada artículo tiene un SKU único, un nombre y una cantidad no negativa:

```json
{
  "sku": "SKU-001",
  "name": "Teclado",
  "quantity": 12
}
```

| Método | Ruta | Descripción |
|---|---|---|
| `GET` | `/items` | Lista los artículos |
| `POST` | `/items` | Crea un artículo; responde `409` si el SKU ya existe |
| `GET` | `/items/{sku}` | Consulta un artículo |
| `PUT` | `/items/{sku}` | Actualiza el artículo; el SKU del cuerpo debe coincidir con la ruta |
| `DELETE` | `/items/{sku}` | Elimina un artículo |
| `GET` | `/health/live` | Indica que el proceso está activo |
| `GET` | `/health/ready` | Comprueba que PostgreSQL responde |
| `GET` | `/health` | Alias de readiness, conservado por compatibilidad |

Ejemplo para crear un artículo:

```bash
curl -X POST http://localhost:8089/items \
  -H 'Content-Type: application/json' \
  -d '{"sku":"SKU-001","name":"Teclado","quantity":12}'
```

Las solicitudes inválidas responden `400`, los recursos inexistentes `404` y los SKU duplicados `409`. Las respuestas de error usan el formato:

```json
{
  "error": "inventory item already exists"
}
```

La especificación OpenAPI está disponible en Swagger UI y en `docs/swagger.json` / `docs/swagger.yaml`.

## Configuración

| Variable | Valor predeterminado | Descripción |
|---|---|---|
| `PORT` | `8089` | Puerto HTTP, entre 1 y 65535 |
| `DATABASE_URL` | `postgres://micro_test:micro_test@localhost:5432/micro_test?sslmode=disable` | DSN de conexión a PostgreSQL |

Para ejecutar el servicio directamente con Go, inicia PostgreSQL por separado y configura el DSN:

```bash
DATABASE_URL='postgres://micro_test:micro_test@localhost:5432/micro_test?sslmode=disable' go run .
```

La tabla de inventario se crea automáticamente al arrancar.

## Desarrollo y validación

```bash
test -z "$(gofmt -l .)"
go test -race ./...
go vet ./...
go build ./...
```

Las pruebas del núcleo no requieren PostgreSQL; usan un repositorio en memoria para comprobar las reglas del servicio y la propagación de errores. El adaptador HTTP se prueba aparte mediante un puerto de aplicación simulado.

Para regenerar la documentación Swagger después de modificar las anotaciones:

```bash
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --generalInfo main.go --dir . --output docs --parseInternal
```

El workflow de GitHub Actions está en `.github/workflows/ci.yml`. Usa Go 1.23.3 y valida el proyecto en los pushes y pull requests a `main`.

## Estructura del proyecto

```text
.
├── docs/                         # OpenAPI generado con Swaggo
├── internal/
│   ├── httpapi/                  # Adaptador de entrada HTTP y sus pruebas
│   ├── inventory/                # Dominio, puerto de salida y servicio de aplicación
│   └── postgres/                 # Adaptador de persistencia y esquema inicial
├── .github/workflows/ci.yml
├── Dockerfile                    # Build multi-stage con runtime mínimo
├── docker-compose.yml            # API y PostgreSQL para desarrollo local
├── go.mod
└── main.go
```
