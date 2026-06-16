package cantonledger

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "canton-bond-platform/pkg/cantonledger/proto/com/daml/ledger/api/v2"
)

func IniciarStreamGRPC(participantURL string, party string, lastOffset *atomic.Int64, onEvent func(payload map[string]any)) error {
	log.Printf("Conectando vía gRPC nativo a %s...", participantURL)

	conn, err := grpc.NewClient(participantURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("fallo al conectar a gRPC: %v", err)
	}
	defer conn.Close()

	client := pb.NewUpdateServiceClient(conn)

	beginOffset := lastOffset.Load()
	req := &pb.GetUpdatesRequest{
		BeginExclusive: beginOffset,
		UpdateFormat: &pb.UpdateFormat{
			IncludeTransactions: &pb.TransactionFormat{
				EventFormat: &pb.EventFormat{
					FiltersByParty: map[string]*pb.Filters{
						party: {
							Cumulative: []*pb.CumulativeFilter{
								{
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

	if beginOffset == 0 {
		log.Println("Stream gRPC abierto (desde el inicio). Esperando eventos...")
	} else {
		log.Printf("Stream gRPC abierto (reanudado desde offset %d). Esperando eventos...", beginOffset)
	}

	for {
		update, err := stream.Recv()
		if err == io.EOF {
			log.Println("El nodo participante cerró el stream.")
			break
		}
		if err != nil {
			return fmt.Errorf("error leyendo datos del stream gRPC: %v", err)
		}

		switch u := update.Update.(type) {
		case *pb.GetUpdatesResponse_Transaction:
			if offset := u.Transaction.GetOffset(); offset > 0 {
				lastOffset.Store(offset)
			}
			for _, event := range u.Transaction.Events {

				if created := event.GetCreated(); created != nil {
					templateName := created.TemplateId.EntityName
					contractId := created.ContractId

					switch templateName {
					case "SimpleHolding":
						log.Printf("💰 [MINT / HOLDING CREADO] Nuevo bono en la red. ContractID: %s...", contractId[:15])
					case "LockedSimpleHolding":
						log.Printf("🔒 [LOCKED HOLDING CREADO] Holding bloqueado por transfer/allocation. ContractID: %s...", contractId[:15])
					case "SimpleTransferInstruction":
						log.Printf("⏳ [TRANSFER PENDIENTE] Nueva transferencia iniciada. ContractID: %s...", contractId[:15])
					case "SimpleAllocation":
						log.Printf("⚙️ [ALLOCATION CREADA] Nueva asignación registrada. ContractID: %s...", contractId[:15])
					case "SimpleTokenRules":
						log.Printf("🏭 [FACTORY] Reglas del Token (Factory) creadas/actualizadas.")
					default:
						log.Printf("📄 [CONTRATO CREADO] Template: %s | ContractID: %s...", templateName, contractId[:15])
					}

					onEvent(map[string]any{
						"action":     "created",
						"contractId": contractId,
						"templateId": templateName,
					})

				} else if archived := event.GetArchived(); archived != nil {
					templateName := archived.TemplateId.EntityName
					contractId := archived.ContractId

					switch templateName {
					case "SimpleHolding":
						log.Printf("🔥 [BURN / CONSUMIDO] Bono quemado o transferido. ContractID: %s...", contractId[:15])
					case "LockedSimpleHolding":
						log.Printf("🔓 [LOCKED HOLDING ARCHIVADO] Holding bloqueado liberado. ContractID: %s...", contractId[:15])
					case "SimpleTransferInstruction":
						log.Printf("✅ [TRANSFER RESUELTO] Transferencia aceptada, rechazada o retirada.")
					case "SimpleAllocation":
						log.Printf("✅ [ALLOCATION RESUELTA] Asignación ejecutada o cancelada.")
					default:
						log.Printf("🗑️ [CONTRATO ARCHIVADO] Template: %s | ContractID: %s...", templateName, contractId[:15])
					}

					onEvent(map[string]any{
						"action":     "archived",
						"contractId": contractId,
						"templateId": templateName,
					})
				}
			}

		case *pb.GetUpdatesResponse_OffsetCheckpoint:
			if offset := u.OffsetCheckpoint.GetOffset(); offset > 0 {
				lastOffset.Store(offset)
			}

		case *pb.GetUpdatesResponse_Reassignment:
			if offset := u.Reassignment.GetOffset(); offset > 0 {
				lastOffset.Store(offset)
			}

		case *pb.GetUpdatesResponse_TopologyTransaction:
			if offset := u.TopologyTransaction.GetOffset(); offset > 0 {
				lastOffset.Store(offset)
			}
		}
	}
	return nil
}
