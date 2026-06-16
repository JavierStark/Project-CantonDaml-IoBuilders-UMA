package cantonledger

import (
	"context"
	"fmt"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "canton-bond-platform/pkg/cantonledger/proto/com/daml/ledger/api/v2"
	adminpb "canton-bond-platform/pkg/cantonledger/proto/com/daml/ledger/api/v2/admin"
)

// GRPCClient talks to the native Canton Ledger API over gRPC.
type GRPCClient struct {
	target  string
	userID  string
	conn    *grpc.ClientConn
	state   pb.StateServiceClient
	cmds    pb.CommandServiceClient
	parties adminpb.PartyManagementServiceClient
}

func NewGRPC(target, userID string, _ time.Duration) (*GRPCClient, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create grpc client: %w", err)
	}
	return &GRPCClient{
		target:  target,
		userID:  userID,
		conn:    conn,
		state:   pb.NewStateServiceClient(conn),
		cmds:    pb.NewCommandServiceClient(conn),
		parties: adminpb.NewPartyManagementServiceClient(conn),
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
	var result []PartyDetail
	pageToken := ""
	for {
		resp, err := c.parties.ListKnownParties(ctx, &adminpb.ListKnownPartiesRequest{
			PageToken: pageToken,
			PageSize:  1000,
		})
		if err != nil {
			return nil, fmt.Errorf("grpc list parties: %w", err)
		}
		for _, p := range resp.GetPartyDetails() {
			result = append(result, PartyDetail{
				Party:   p.GetParty(),
				IsLocal: p.GetIsLocal(),
			})
		}
		pageToken = resp.GetNextPageToken()
		if pageToken == "" {
			break
		}
	}
	if result == nil {
		return []PartyDetail{}, nil
	}
	return result, nil
}

func (c *GRPCClient) AllocateParty(ctx context.Context, hint string) (PartyDetail, error) {
	resp, err := c.parties.AllocateParty(ctx, &adminpb.AllocatePartyRequest{
		PartyIdHint: hint,
		UserId:      c.userID,
	})
	if err != nil {
		return PartyDetail{}, fmt.Errorf("grpc allocate party: %w", err)
	}
	p := resp.GetPartyDetails()
	if p == nil {
		return PartyDetail{}, fmt.Errorf("grpc allocate party returned empty party details")
	}
	return PartyDetail{
		Party:   p.GetParty(),
		IsLocal: p.GetIsLocal(),
	}, nil
}

func (c *GRPCClient) SubmitCommand(ctx context.Context, commandID string, cmd Command, actAs []string) (int64, error) {
	pbCmd, err := grpcCommand(cmd)
	if err != nil {
		return 0, err
	}
	resp, err := c.cmds.SubmitAndWait(ctx, &pb.SubmitAndWaitRequest{
		Commands: &pb.Commands{
			UserId:    c.userID,
			CommandId: commandID,
			ActAs:     actAs,
			ReadAs:    actAs,
			Commands:  []*pb.Command{pbCmd},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("grpc submit-and-wait: %w", err)
	}
	return resp.GetCompletionOffset(), nil
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

func grpcCommand(cmd Command) (*pb.Command, error) {
	switch {
	case cmd.CreateCommand != nil:
		templateID, err := identifierFromString(cmd.CreateCommand.TemplateID)
		if err != nil {
			return nil, err
		}
		args, err := encodeCreateRecord(cmd.CreateCommand.CreateArguments)
		if err != nil {
			return nil, err
		}
		return &pb.Command{
			Command: &pb.Command_Create{
				Create: &pb.CreateCommand{
					TemplateId:      templateID,
					CreateArguments: args,
				},
			},
		}, nil
	case cmd.ExerciseCommand != nil:
		templateID, err := identifierFromString(cmd.ExerciseCommand.TemplateID)
		if err != nil {
			return nil, err
		}
		arg, err := encodeChoiceArgument(cmd.ExerciseCommand.ChoiceArgument)
		if err != nil {
			return nil, err
		}
		return &pb.Command{
			Command: &pb.Command_Exercise{
				Exercise: &pb.ExerciseCommand{
					TemplateId:     templateID,
					ContractId:     cmd.ExerciseCommand.ContractID,
					Choice:         cmd.ExerciseCommand.Choice,
					ChoiceArgument: arg,
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("empty command")
	}
}
