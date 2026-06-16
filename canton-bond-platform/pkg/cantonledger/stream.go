package cantonledger

import (
	"context"
	"fmt"
	"io"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "canton-bond-platform/pkg/cantonledger/proto/com/daml/ledger/api/v2"
)

// IniciarStreamGRPC ahora acepta una función 'onEvent' para inyectar los eventos al main.go
func IniciarStreamGRPC(participantURL string, party string, onEvent func(payload map[string]any)) error {
	log.Printf("Conectando vía gRPC nativo a %s...", participantURL)

	conn, err := grpc.NewClient(participantURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("fallo al conectar a gRPC: %v", err)
	}
	defer conn.Close()

	client := pb.NewUpdateServiceClient(conn)

	// Petición actualizada con el WildcardFilter obligatorio para Canton v3 / Daml 2.x
	req := &pb.GetUpdatesRequest{
		BeginExclusive: 0,
		UpdateFormat: &pb.UpdateFormat{
			IncludeTransactions: &pb.TransactionFormat{
				EventFormat: &pb.EventFormat{
					FiltersByParty: map[string]*pb.Filters{
						party: {
							Cumulative: []*pb.CumulativeFilter{
								{
									// El campo de la interfaz se llama IdentifierFilter,
									// pero el tipo en Go es CumulativeFilter_WildcardFilter
									IdentifierFilter: &pb.CumulativeFilter_WildcardFilter{
										WildcardFilter: &pb.WildcardFilter{
											IncludeCreatedEventBlob: false,
										},
									},
								},
							},
						},
					},
				},
				TransactionShape: pb.TransactionShape_TRANSACTION_SHAPE_LEDGER_EFFECTS,
			},
		},
	}

	stream, err := client.GetUpdates(context.Background(), req)
	if err != nil {
		return fmt.Errorf("fallo al abrir el stream gRPC: %v", err)
	}

	log.Println("Stream gRPC abierto. Esperando eventos para enviar a los WebSockets...")

	for {
		update, err := stream.Recv()
		if err == io.EOF {
			log.Println("El nodo participante cerró el stream.")
			break
		}
		if err != nil {
			return fmt.Errorf("error leyendo datos del stream gRPC: %v", err)
		}

		// Filtramos silenciosamente solo lo que nos interesa (Transacciones reales)
		switch u := update.Update.(type) {
		case *pb.GetUpdatesResponse_Transaction:
			for _, event := range u.Transaction.Events {

				// 🟢 CASO 1: SE HA CREADO UN CONTRATO NUEVO (Mint, Pendientes, etc.)
				if created := event.GetCreated(); created != nil {
					templateName := created.TemplateId.EntityName
					contractId := created.ContractId

					// Logueamos de forma elegante en el Backend (Terminal)
					switch templateName {
					case "SimpleHolding":
						log.Printf("💰 [MINT / HOLDING CREADO] Nuevo bono en la red. ContractID: %s...", contractId[:15])
					case "TransferInstruction": // Ajusta este nombre si tu Daml lo llama distinto
						log.Printf("⏳ [TRANSFER PENDIENTE] Nueva transferencia iniciada. ContractID: %s...", contractId[:15])
					case "AllocationInstruction":
						log.Printf("⚙️ [ALLOCATION CREADA] Nueva asignación registrada. ContractID: %s...", contractId[:15])
					case "SimpleTokenRules":
						log.Printf("🏭 [FACTORY] Reglas del Token (Factory) creadas/actualizadas.")
					default:
						log.Printf("📄 [CONTRATO CREADO] Template: %s | ContractID: %s...", templateName, contractId[:15])
					}

					// Enviamos el aviso al Frontend por WebSocket
					payload := map[string]any{
						"action":     "created",
						"contractId": contractId,
						"templateId": templateName,
					}
					onEvent(payload)

					// 🔴 CASO 2: SE HA CONSUMIDO/ARCHIVADO UN CONTRATO (Burn, Transfer aceptado, etc.)
				} else if archived := event.GetArchived(); archived != nil {
					templateName := archived.TemplateId.EntityName
					contractId := archived.ContractId

					// Logueamos de forma elegante en el Backend (Terminal)
					switch templateName {
					case "SimpleHolding":
						log.Printf("🔥 [BURN / CONSUMIDO] Bono quemado o transferido. ContractID: %s...", contractId[:15])
					case "TransferInstruction":
						log.Printf("✅ [TRANSFER RESUELTO] Transferencia aceptada, rechazada o retirada.")
					case "AllocationInstruction":
						log.Printf("✅ [ALLOCATION RESUELTA] Asignación ejecutada o cancelada.")
					default:
						log.Printf("🗑️ [CONTRATO ARCHIVADO] Template: %s | ContractID: %s...", templateName, contractId[:15])
					}

					// Enviamos el aviso al Frontend por WebSocket
					payload := map[string]any{
						"action":     "archived",
						"contractId": contractId,
						"templateId": templateName,
					}
					onEvent(payload)
				}
			}
		}
	}
	return nil
}
