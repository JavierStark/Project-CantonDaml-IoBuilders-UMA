package cantonledger

// Bond contract template IDs (#package-name:Module:Entity).
const (
	TemplateSimpleTokenRules          = "#simple-token:SimpleToken.Rules:SimpleTokenRules"
	TemplateSimpleHolding             = "#simple-token:SimpleToken.Holding:SimpleHolding"
	TemplateLockedSimpleHolding       = "#simple-token:SimpleToken.Holding:LockedSimpleHolding"
	TemplateSimpleTransferInstruction = "#simple-token:SimpleToken.TransferInstruction:SimpleTransferInstruction"
	TemplateSimpleAllocation          = "#simple-token:SimpleToken.Allocation:SimpleAllocation"
)

// BondTemplates is the default set monitored by the ledger listener.
func BondTemplates() []string {
	return []string{
		TemplateSimpleTokenRules,
		TemplateSimpleHolding,
		TemplateLockedSimpleHolding,
		TemplateSimpleTransferInstruction,
		TemplateSimpleAllocation,
	}
}

// CIP-056 interface template IDs (full package hash).
const (
	TemplateTransferFactory     = "55ba4deb0ad4662c4168b39859738a0e91388d252286480c7331b3f71a517281:Splice.Api.Token.TransferInstructionV1:TransferFactory"
	TemplateTransferInstruction = "55ba4deb0ad4662c4168b39859738a0e91388d252286480c7331b3f71a517281:Splice.Api.Token.TransferInstructionV1:TransferInstruction"
	TemplateAllocationFactory   = "275064aacfe99cea72ee0c80563936129563776f67415ef9f13e4297eecbc520:Splice.Api.Token.AllocationInstructionV1:AllocationFactory"
	TemplateAllocation          = "93c942ae2b4c2ba674fb152fe38473c507bda4e82b4e4c5da55a552a9d8cce1d:Splice.Api.Token.AllocationV1:Allocation"
)

// Interface choice names for JSON API ExerciseCommand.
const (
	ChoiceAddObserver                 = "AddObserver"
	ChoiceMint                        = "Mint"
	ChoiceTransferOwnership           = "TransferOwnership"
	ChoiceBurn                        = "Burn"
	ChoiceBurnByAdmin                 = "BurnByAdmin"
	ChoiceTransferFactoryTransfer     = "TransferFactory_Transfer"
	ChoiceTransferInstructionAccept   = "TransferInstruction_Accept"
	ChoiceTransferInstructionReject   = "TransferInstruction_Reject"
	ChoiceTransferInstructionWithdraw = "TransferInstruction_Withdraw"
	ChoiceLockedSimpleHoldingUnlock   = "LockedSimpleHolding_Unlock"
	ChoiceAllocationFactoryAllocate   = "AllocationFactory_Allocate"
	ChoiceAllocationExecute           = "Allocation_ExecuteTransfer"
	ChoiceAllocationCancel            = "Allocation_Cancel"
	ChoiceAllocationWithdraw          = "Allocation_Withdraw"
)
