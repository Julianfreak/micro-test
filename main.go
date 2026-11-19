package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// HealthResponse define la estructura de la respuesta para el endpoint de salud
type HealthResponse struct {
	Status string `json:"status"`
	Agent  string `json:"agent"`
}

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
	
	http.HandleFunc("/health", healthHandler)

	fmt.Printf("Servidor iniciado en el puerto %s...\n", port)
	fmt.Printf("Endpoint disponible: http://localhost:%s/health\n", port)

	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalf("Error al iniciar el servidor: %s\n", err)
	}
}
