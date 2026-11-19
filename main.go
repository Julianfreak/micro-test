package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	_ "github.com/Julianfreak/micro-test/docs" // Importación necesaria para Swagger
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title           Micro-Test API
// @version         1.0
// @description     Servicio de salud para la infraestructura de microservicios robustos en Go.
// @termsOfService  http://swagger.io/terms/

// @contact.name   Soporte Técnico
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8089
// @BasePath  /

// HealthResponse representa la respuesta de estado del sistema
type HealthResponse struct {
	Status string `json:"status" example:"UP"`
	Agent  string `json:"agent" example:"local"`
}

// HealthHandler godoc
// @Summary      Verificar el estado del servicio
// @Description  Obtiene el estado actual y el agente del microservicio
// @Tags         health
// @Produce      json
// @Success      200  {object}  HealthResponse
// @Router       /health [get]
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := HealthResponse{
		Status: "UP",
		Agent:  "local",
	}
	json.NewEncoder(w).Encode(response)
}

func main() {
	port := "8089"

	// Rutas
	http.HandleFunc("/health", healthHandler)
	http.Handle("/swagger/", httpSwagger.WrapHandler)

	fmt.Printf("Servicio Profesional iniciado en el puerto %s...\n", port)
	fmt.Printf("Documentación Swagger: http://localhost:%s/swagger/index.html\n", port)

	server := &http.Server{
		Addr: ":" + port,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Error crítico en el servidor: %s\n", err)
	}
}
