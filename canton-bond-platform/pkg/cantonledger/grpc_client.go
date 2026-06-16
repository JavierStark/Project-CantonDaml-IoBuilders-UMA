package cantonledger

import (
	"context"
	"fmt"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "canton-bond-platform/pkg/cantonledger/proto/com/daml/ledger/api/v2"
)

// GRPCClient talks to the native Canton Ledger API over gRPC for state reads,
// and falls back to the HTTP JSON API for writes and party management.
type GRPCClient struct {
	target     string
	userID     string
	conn       *grpc.ClientConn
	state      pb.StateServiceClient
	httpClient *Client
}

func NewGRPC(target, httpURL, userID string, timeout time.Duration) (*GRPCClient, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create grpc ledger client: %w", err)
	}

	return &GRPCClient{
		target:     target,
		userID:     userID,
		conn:       conn,
		state:      pb.NewStateServiceClient(conn),
		httpClient: New(httpURL, userID, timeout),
	}, nil
}

func (c *GRPCClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *GRPCClient) LedgerEnd(ctx context.Context) (int64, error) {
	resp, err := c.state.GetLedgerEnd(ctx, &pb.GetLedgerEndRequest{})
	if err != nil {
		return 0, fmt.Errorf("grpc ledger-end: %w", err)
	}
	return resp.GetOffset(), nil
}

func (c *GRPCClient) ActiveContracts(ctx context.Context, offset int64, _ ...string) (ActiveContractsResponse, error) {
	stream, err := c.state.GetActiveContracts(ctx, &pb.GetActiveContractsRequest{
		ActiveAtOffset: offset,
		EventFormat:    grpcWildcardEventFormat(),
	})
	if err != nil {
		return nil, fmt.Errorf("grpc active-contracts: %w", err)
	}

	var out ActiveContractsResponse
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("grpc active-contracts recv: %w", err)
		}
		active := resp.GetActiveContract()
		if active == nil || active.GetCreatedEvent() == nil {
			continue
		}
		out = append(out, grpcCreatedEventEntry(active.GetCreatedEvent()))
	}
	if out == nil {
		return ActiveContractsResponse{}, nil
	}
	return out, nil
}

func (c *GRPCClient) Parties(ctx context.Context) ([]PartyDetail, error) {
	return c.httpClient.Parties(ctx)
}

func (c *GRPCClient) AllocateParty(ctx context.Context, hint string) (PartyDetail, error) {
	return c.httpClient.AllocateParty(ctx, hint)
}

func (c *GRPCClient) SubmitCommand(ctx context.Context, commandID string, cmd Command, actAs []string) (int64, error) {
	return c.httpClient.SubmitCommand(ctx, commandID, cmd, actAs)
}

func grpcWildcardEventFormat() *pb.EventFormat {
	return &pb.EventFormat{
		FiltersForAnyParty: &pb.Filters{
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
		Verbose: true,
	}
}

func grpcCreatedEventEntry(evt *pb.CreatedEvent) map[string]any {
	return map[string]any{
		"createdEvent": map[string]any{
			"contractId":     evt.GetContractId(),
			"templateId":     identifierString(evt.GetTemplateId()),
			"createArgument": recordToMap(evt.GetCreateArguments()),
		},
	}
}
