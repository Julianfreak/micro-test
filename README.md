# Micro-Test Service

Microservicio profesional desarrollado en Go, diseñado bajo principios de escalabilidad, observabilidad y entrega continua.

## 🚀 Características

- **Arquitectura Limpia**: Implementación robusta utilizando el toolkit estándar de Go.
- **Documentación Interactiva**: Integración nativa con OpenAPI/Swagger.
- **Contenedorización**: Dockerfile multi-stage optimizado para imágenes de producción de bajo footprint.
- **CI/CD**: Pipeline automatizado con GitHub Actions para validación de código y builds de contenedores.
- **Observabilidad**: Endpoint de salud nativo para orquestadores como Kubernetes.

## 🛠️ Tecnologías

- **Go**: 1.21+
- **OpenAPI/Swagger**: Para documentación de API.
- **Docker**: Despliegue mediante contenedores.
- **GitHub Actions**: Automatización de CI/CD.

## 📦 Ejecución con Docker

Para levantar el servicio en un entorno contenedorizado:

```bash
docker build -t micro-test .
docker run -p 8089:8089 micro-test
```

## 📖 Documentación de la API

Una vez iniciado el servicio, la documentación interactiva de Swagger está disponible en:
[http://localhost:8089/swagger/index.html](http://localhost:8089/swagger/index.html)

## 🚦 Endpoints Principales

### Health Check
- **URL**: `/health`
- **Descripción**: Indica el estado operativo del servicio.
- **Respuesta**: `{"status": "UP", "agent": "local"}`

## ⚙️ CI/CD Pipeline

El proyecto incluye un flujo de trabajo en `.github/workflows/ci.yml` que ejecuta automáticamente:
1. Instalación de dependencias.
2. Compilación del binario.
3. Ejecución de pruebas unitarias.
4. Verificación de construcción de imagen Docker.
