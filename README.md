# Micro-Test Service

Microservicio desarrollado en Go, diseñado bajo principios de simplicidad, escalabilidad, observabilidad y entrega continua.

## 🚀 Características

- **Arquitectura ligera:** Implementación utilizando el paquete estándar `net/http` de Go.
- **Documentación interactiva:** Integración con OpenAPI/Swagger mediante Swaggo.
- **Contenedorización:** Docker con imagen mínima `scratch` para reducir el tamaño y la superficie de ataque.
- **Orquestación local:** Docker Compose para facilitar la construcción y ejecución del servicio.
- **CI/CD:** Pipeline automatizado con GitHub Actions para validación de código y construcción de contenedores.
- **Observabilidad:** Endpoint de salud para verificar el estado operativo del servicio.

## 🛠️ Tecnologías

- **Go:** 1.23.3.
- **OpenAPI/Swagger:** Documentación interactiva de la API.
- **Docker:** Construcción y ejecución de contenedores.
- **Docker Compose:** Administración del servicio.
- **GitHub Actions:** Automatización de CI/CD.

## 📁 Estructura del proyecto

```text
microservicios/
├── docs/
│   ├── docs.go
│   ├── swagger.json
│   └── swagger.yaml
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
├── main.go
├── micro-test
└── README.md
```

## 📦 Ejecución con Docker

El proyecto utiliza una imagen Docker basada en `scratch`, que permite ejecutar el binario sin incorporar un sistema operativo completo ni dependencias dinámicas.

Para utilizar esta configuración, primero es necesario compilar la aplicación localmente.

### 1. Compilar el binario

Desde la raíz del proyecto ejecutar:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o micro-test -ldflags="-w -s" main.go
```

**Explicación:**

- `CGO_ENABLED=0`: genera un ejecutable sin dependencias dinámicas de C.
- `GOOS=linux`: establece Linux como sistema operativo de destino.
- `GOARCH=amd64`: establece la arquitectura x86-64.
- `-ldflags="-w -s"`: elimina información de depuración y símbolos para reducir el tamaño del ejecutable.

### 2. Construir e iniciar el contenedor

Desde la raíz del proyecto ejecutar:

```bash
docker compose up --build -d
```

Docker Compose construirá la imagen utilizando el Dockerfile y ejecutará el servicio.

### 3. Verificar el contenedor

Consultar el estado:

```bash
docker compose ps
```

Consultar los registros:

```bash
docker compose logs -f micro-test
```

### 4. Detener el servicio

```bash
docker compose down
```

### 5. Reconstruir después de realizar cambios

Cuando se modifique el código fuente, es necesario volver a compilar el binario antes de construir la imagen:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o micro-test -ldflags="-w -s" main.go

docker compose up --build -d
```

**Nota:** La configuración actual utiliza un binario compilado para Linux AMD64. Si el entorno de destino utiliza otra arquitectura, será necesario ajustar `GOARCH`.

## 📖 Documentación de la API

Una vez iniciado el servicio, la documentación interactiva de Swagger estará disponible en:

[http://localhost:8089/swagger/index.html](http://localhost:8089/swagger/index.html)

## 🚦 Endpoints principales

### Health Check

- **Método:** GET
- **URL:** `/health`
- **Descripción:** Permite verificar el estado operativo del microservicio.

Respuesta esperada:

```json
{
  "status": "UP",
  "agent": "local"
}
```

Para probar el endpoint desde la terminal:

```bash
curl http://localhost:8089/health
```

## ⚙️ CI/CD Pipeline

El proyecto incluye un flujo de trabajo en `.github/workflows/ci.yml` que automatiza las siguientes tareas:

1. Instalación de dependencias.
2. Compilación del código fuente.
3. Ejecución de pruebas unitarias.
4. Verificación de construcción de la imagen Docker.

## 🔐 Consideraciones de despliegue

- El contenedor utiliza `scratch` como imagen base para minimizar componentes innecesarios.
- El binario debe compilarse para Linux y la arquitectura correspondiente al entorno de ejecución.
- Se utiliza `CGO_ENABLED=0` para generar un ejecutable independiente de bibliotecas dinámicas de C.
- Docker Compose publica el puerto `8089` del contenedor en el puerto `8089` del host.