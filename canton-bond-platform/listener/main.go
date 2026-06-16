package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

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

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func LoadConfig() Config {
	return Config{
		ParticipantURL: getEnv("LISTENER_PARTICIPANT_URL", "participant1:5011"),
		Party:          getEnv("LISTENER_PARTY", "admin"),
	}
}

func getWSPort() string {
	return getEnv("LISTENER_WS_PORT", "8081")
}

// --- NUEVA FUNCIÓN DE AUTO-DESCUBRIMIENTO ---
// Utiliza tu client.go y parties.go para buscar el hash real en vivo
func resolverPartyID(httpURL string, shortName string) string {
	log.Printf("Buscando el hash criptográfico real para la party: '%s'...", shortName)
	client := cantonledger.New(httpURL, "", 10*time.Second)

	for {
		parties, err := client.Parties(context.Background())
		if err == nil {
			for _, p := range parties {
				// Canton siempre guarda las parties con el formato "nombre::hash"
				if strings.HasPrefix(p.Party, shortName+"::") {
					return p.Party
				}
			}
		}
		log.Printf("Canton aún no ha generado la party '%s'. Reintentando en 5 segundos...", shortName)
		time.Sleep(5 * time.Second)
	}
}

func main() {
	cfg := LoadConfig()

	// 0. AUTO-DESCUBRIMIENTO DEL ID ANTES DE ARRANCAR gRPC
	httpBaseURL := getEnv("LISTENER_HTTP_URL", "http://participant1:5013")
	realPartyID := resolverPartyID(httpBaseURL, cfg.Party)

	log.Printf("¡ÉXITO! ID resuelto: %s -> %s", cfg.Party, realPartyID)
	cfg.Party = realPartyID // Sustituimos "admin" por el hash real ("admin::1220...")

	log.Printf("Bond Listener Iniciado con gRPC (participant=%s)", cfg.ParticipantURL)

	// Track the last seen offset for reconnection resume
	var lastOffset atomic.Int64

	// 1. Arrancamos el Stream gRPC en una Goroutine paralela
	go func() {
		for {
			log.Printf("Conectando vía gRPC nativo a %s...", cfg.ParticipantURL)

			err := cantonledger.IniciarStreamGRPC(cfg.ParticipantURL, cfg.Party, &lastOffset, func(payload map[string]any) {
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

			if err != nil {
				log.Printf("[Stream Error]: %v", err)
			}
			log.Println("Canton desconectado. Reintentando en 5 segundos...")
			time.Sleep(5 * time.Second)
		}
	}()

	// 2. Endpoint HTTP para WebSockets
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

		// Read loop: detect client disconnection via close frames
		go func() {
			defer func() {
				clientsMu.Lock()
				delete(wsClients, conn)
				clientsMu.Unlock()
				conn.Close()
				log.Println("[WS] Cliente desconectado y eliminado.")
			}()
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					break
				}
			}
		}()
	})

	// 3. Levantamos el servidor
	wsPort := getWSPort()
	go func() {
		log.Printf("Servidor WebSocket a la escucha en el puerto :%s/ws/bonds", wsPort)
		if err := http.ListenAndServe(":"+wsPort, nil); err != nil {
			log.Fatalf("Fallo crítico en servidor HTTP: %v", err)
		}
	}()

	// Bloqueo para mantener vivo el programa
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Println("Apagando Bond Listener...")
}
