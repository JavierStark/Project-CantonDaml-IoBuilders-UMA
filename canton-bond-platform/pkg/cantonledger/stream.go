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
	// Estructura plana y correcta para Canton v3 (API v2)
	req := &pb.GetUpdatesRequest{
		BeginExclusive: 0,
		UpdateFormat: &pb.UpdateFormat{
			IncludeTransactions: &pb.TransactionFormat{
				EventFormat: &pb.EventFormat{
					FiltersByParty: map[string]*pb.Filters{
						party: {
							// Esta estructura es la más compatible con Canton 3.x
							// Si la party es válida, Canton enviará los eventos automáticamente.
							Cumulative: []*pb.CumulativeFilter{},
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
			log.Println("El nodo cerró el stream.")
			break
		}
		if err != nil {
			return fmt.Errorf("error en el stream gRPC: %v", err)
		}

		switch u := update.Update.(type) {
		case *pb.GetUpdatesResponse_Transaction:
			for _, event := range u.Transaction.Events {
				if created := event.GetCreated(); created != nil {
					// Extraemos los datos gRPC y creamos un payload simple para el frontend
					payload := map[string]any{
						"action":     "created",
						"contractId": created.ContractId,
						"templateId": created.TemplateId.EntityName,
					}
					onEvent(payload) // ¡Disparamos el evento al main.go!
				}
				if archived := event.GetArchived(); archived != nil {
					payload := map[string]any{
						"action":     "archived",
						"contractId": archived.ContractId,
						"templateId": archived.TemplateId.EntityName,
					}
					onEvent(payload)
				}
			}
		}
	}
	return nil
}
