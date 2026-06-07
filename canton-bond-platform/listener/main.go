package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time" // Añadido para controlar el tiempo de espera entre reintentos

	"canton-bond-platform/pkg/cantonledger"

	"github.com/gorilla/websocket"
)

// --- CONFIGURACIÓN WEBSOCKET ---
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var wsClients = make(map[*websocket.Conn]bool)
var clientsMu sync.Mutex

type Config struct {
	ParticipantURL string
	Party          string
}

func LoadConfig() Config {
	return Config{
		ParticipantURL: getEnv("LISTENER_PARTICIPANT_URL", "localhost:5011"), // Puerto 5011 para gRPC
		Party:          getEnv("LISTENER_PARTY", "admin"),
	}
}

func main() {
	cfg := LoadConfig()
	log.Printf("Bond Listener Iniciado con gRPC (participant=%s)", cfg.ParticipantURL)

	// 1. Arrancamos el Stream gRPC en una Goroutine paralela con lógica de reintento masivo
	go func() {
		for {
			log.Printf("Conectando vía gRPC nativo a %s...", cfg.ParticipantURL)

			// Ejecutamos el stream pasándole el callback para propagar eventos
			err := cantonledger.IniciarStreamGRPC(cfg.ParticipantURL, cfg.Party, func(payload map[string]any) {
				// Lo imprimimos bonito en la consola
				data, _ := json.Marshal(payload)
				log.Printf("Evento gRPC propagado: %s", data)

				// ¡MAGIA! Lo inyectamos directamente en todos los frontends conectados
				clientsMu.Lock()
				for conn := range wsClients {
					if err := conn.WriteJSON(payload); err != nil {
						log.Printf("[WS] Error enviando a cliente, desconectando: %v", err)
						conn.Close()
						delete(wsClients, conn)
					}
				}
				clientsMu.Unlock()
			})

			// Si llega aquí, significa que la conexión ha fallado o se ha cortado
			if err != nil {
				log.Printf("[Stream Error]: %v", err)
			}

			log.Println("Canton podría estar inicializándose o desconectado. Reintentando en 5 segundos...")
			time.Sleep(5 * time.Second) // Espera antes de volver a intentar el bucle
		}
	}()

	// 2. Endpoint HTTP para aceptar conexiones WebSocket entrantes de tu Frontend
	http.HandleFunc("/ws/bonds", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Fallo al actualizar a WebSocket: %v", err)
			return
		}
		clientsMu.Lock()
		wsClients[conn] = true
		clientsMu.Unlock()
		log.Println("[WS] ¡Nuevo cliente Frontend conectado!")
	})

	// 3. Levantamos el servidor de WebSockets
	go func() {
		log.Println("Servidor WebSocket a la escucha en el puerto :8081/ws/bonds")
		if err := http.ListenAndServe(":8081", nil); err != nil {
			log.Fatalf("Fallo crítico en servidor HTTP: %v", err)
		}
	}()

	// 4. Esperar señal para apagado limpio
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Apagando Bond Listener...")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
