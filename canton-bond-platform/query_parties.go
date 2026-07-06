package main

import (
	"context"
	"fmt"
	"log"

	"canton-bond-platform/backend/cantonledger"
)

func main() {
	client, err := cantonledger.New("localhost:5011", "admin", 10)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()
	offset, _ := client.LedgerEnd(ctx)
	resp, err := client.ActiveContracts(ctx, offset, cantonledger.TemplateSimpleTokenRules)
	if err != nil {
		log.Fatal(err)
	}
	for _, evt := range resp.CreatedEvents {
		fmt.Printf("Factory CID: %s\n", evt.ContractId)
		if obs, ok := evt.CreateArguments.Fields["observers"]; ok {
			fmt.Printf("Observers: %+v\n", obs)
		}
	}
}
