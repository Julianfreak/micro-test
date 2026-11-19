# Micro-Test Go Service

Este es un microservicio básico desarrollado en Go como parte de una prueba técnica de arquitectura.

## Componentes del Proyecto

- **Languaje**: Go (Golang)
- **Framework**: Estándar de la biblioteca `net/http` de Go.
- **Endpoints**:
  - `GET /health`: Devuelve el estado actual del servicio.

## Estructura de Archivos

- `main.go`: Contiene la lógica principal del servidor HTTP y la definición del endpoint de salud.
- `go.mod`: Definición del módulo y dependencias.

## Ejecución Local

Para ejecutar el proyecto localmente, asegúrate de tener Go instalado y ejecuta:

```bash
go run main.go
```

El servidor estará disponible en [http://localhost:8089](http://localhost:8089).

## Endpoint de Salud

- **URL**: `/health`
- **Método**: `GET`
- **Respuesta**:
  ```json
  {
    "status": "UP",
    "agent": "local"
  }
  ```
