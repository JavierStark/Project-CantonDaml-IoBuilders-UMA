package cantonledger

import "math/big"

// DamlDecimal formats a float as a Daml decimal string.
func DamlDecimal(v float64) string {
	return big.NewFloat(v).Text('f', 10)
}

// DamlDate passes through a date string for Daml Date fields.
func DamlDate(date string) string {
	return date
}

// InstrumentID builds a CIP instrumentId record.
func InstrumentID(adminParty, code string) map[string]any {
	return map[string]any{
		"admin": adminParty,
		"id":    code,
	}
}
