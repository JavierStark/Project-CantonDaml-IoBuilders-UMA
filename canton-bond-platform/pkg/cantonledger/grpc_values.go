package cantonledger

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	pb "canton-bond-platform/pkg/cantonledger/proto/com/daml/ledger/api/v2"
)

func identifierFromString(id string) (*pb.Identifier, error) {
	parts := strings.Split(id, ":")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid identifier %q", id)
	}
	return &pb.Identifier{
		PackageId:  parts[0],
		ModuleName: parts[1],
		EntityName: parts[2],
	}, nil
}

func identifierString(id *pb.Identifier) string {
	if id == nil {
		return ""
	}
	return fmt.Sprintf("%s:%s:%s", id.PackageId, id.ModuleName, id.EntityName)
}

func encodeCreateRecord(v any) (*pb.Record, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("create arguments must be an object, got %T", v)
	}
	return encodeRecord(m)
}

func encodeRecord(m map[string]any) (*pb.Record, error) {
	fields := make([]*pb.RecordField, 0, len(m))
	for k, v := range m {
		value, err := encodeValue(k, v)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		fields = append(fields, &pb.RecordField{Label: k, Value: value})
	}
	return &pb.Record{Fields: fields}, nil
}

func encodeChoiceArgument(v any) (*pb.Value, error) {
	if m, ok := v.(map[string]any); ok && len(m) == 0 {
		return &pb.Value{Sum: &pb.Value_Record{Record: &pb.Record{}}}, nil
	}
	return encodeValue("", v)
}

func encodeValue(field string, v any) (*pb.Value, error) {
	if v == nil {
		return &pb.Value{Sum: &pb.Value_Optional{Optional: &pb.Optional{}}}, nil
	}

	switch t := v.(type) {
	case string:
		return encodeStringValue(field, t)
	case bool:
		return &pb.Value{Sum: &pb.Value_Bool{Bool: t}}, nil
	case int:
		return &pb.Value{Sum: &pb.Value_Int64{Int64: int64(t)}}, nil
	case int64:
		return &pb.Value{Sum: &pb.Value_Int64{Int64: t}}, nil
	case float64:
		return &pb.Value{Sum: &pb.Value_Numeric{Numeric: DamlDecimal(t)}}, nil
	case []string:
		elements := make([]*pb.Value, 0, len(t))
		for _, item := range t {
			value, err := encodeStringListItem(field, item)
			if err != nil {
				return nil, err
			}
			elements = append(elements, value)
		}
		return &pb.Value{Sum: &pb.Value_List{List: &pb.List{Elements: elements}}}, nil
	case []any:
		elements := make([]*pb.Value, 0, len(t))
		for _, item := range t {
			value, err := encodeValue(field, item)
			if err != nil {
				return nil, err
			}
			elements = append(elements, value)
		}
		return &pb.Value{Sum: &pb.Value_List{List: &pb.List{Elements: elements}}}, nil
	case map[string]any:
		if field == "values" {
			entries := make([]*pb.TextMap_Entry, 0, len(t))
			for k, raw := range t {
				value, err := encodeValue(k, raw)
				if err != nil {
					return nil, err
				}
				entries = append(entries, &pb.TextMap_Entry{Key: k, Value: value})
			}
			return &pb.Value{Sum: &pb.Value_TextMap{TextMap: &pb.TextMap{Entries: entries}}}, nil
		}
		rec, err := encodeRecord(t)
		if err != nil {
			return nil, err
		}
		return &pb.Value{Sum: &pb.Value_Record{Record: rec}}, nil
	default:
		return nil, fmt.Errorf("unsupported value type %T", v)
	}
}

func encodeStringValue(field, value string) (*pb.Value, error) {
	if field == "cid" {
		return &pb.Value{Sum: &pb.Value_Optional{Optional: &pb.Optional{
			Value: &pb.Value{Sum: &pb.Value_ContractId{ContractId: value}},
		}}}, nil
	}
	if isPartyField(field) {
		return &pb.Value{Sum: &pb.Value_Party{Party: value}}, nil
	}
	if isNumericField(field) {
		return &pb.Value{Sum: &pb.Value_Numeric{Numeric: value}}, nil
	}
	if isDateField(field) {
		days, err := encodeDamlDate(value)
		if err != nil {
			return nil, err
		}
		return &pb.Value{Sum: &pb.Value_Date{Date: days}}, nil
	}
	if isTimestampField(field) {
		micros, err := encodeDamlTimestamp(value)
		if err != nil {
			return nil, err
		}
		return &pb.Value{Sum: &pb.Value_Timestamp{Timestamp: micros}}, nil
	}
	if isContractIDField(field) {
		return &pb.Value{Sum: &pb.Value_ContractId{ContractId: value}}, nil
	}
	return &pb.Value{Sum: &pb.Value_Text{Text: value}}, nil
}

func encodeStringListItem(field, value string) (*pb.Value, error) {
	switch {
	case isPartyListField(field):
		return &pb.Value{Sum: &pb.Value_Party{Party: value}}, nil
	case isContractIDListField(field):
		return &pb.Value{Sum: &pb.Value_ContractId{ContractId: value}}, nil
	default:
		return &pb.Value{Sum: &pb.Value_Text{Text: value}}, nil
	}
}

func isPartyField(field string) bool {
	switch field {
	case "admin", "expectedAdmin", "owner", "sender", "receiver", "executor", "newOwner":
		return true
	default:
		return false
	}
}

func isPartyListField(field string) bool {
	return field == "observers" || field == "holders" || field == "extraObservers"
}

func isNumericField(field string) bool {
	switch field {
	case "amount", "couponRate":
		return true
	default:
		return false
	}
}

func isDateField(field string) bool {
	return field == "maturityDate"
}

func isTimestampField(field string) bool {
	switch field {
	case "requestedAt", "executeBefore", "allocateBefore", "settleBefore", "expiresAt":
		return true
	default:
		return false
	}
}

func isContractIDField(field string) bool {
	return strings.HasSuffix(field, "Cid") || strings.HasSuffix(field, "CID") || field == "contractId"
}

func isContractIDListField(field string) bool {
	return strings.HasSuffix(field, "Cids") || strings.HasSuffix(field, "CIDs")
}

func encodeDamlDate(value string) (int32, error) {
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return 0, err
	}
	epoch := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	return int32(t.Sub(epoch).Hours() / 24), nil
}

func encodeDamlTimestamp(value string) (int64, error) {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return 0, err
	}
	return t.UTC().UnixNano() / int64(time.Microsecond), nil
}

func valueToAny(v *pb.Value) any {
	if v == nil {
		return nil
	}
	switch s := v.Sum.(type) {
	case *pb.Value_Unit:
		return map[string]any{}
	case *pb.Value_Bool:
		return s.Bool
	case *pb.Value_Int64:
		return s.Int64
	case *pb.Value_Date:
		return decodeDamlDate(s.Date)
	case *pb.Value_Timestamp:
		return decodeDamlTimestamp(s.Timestamp)
	case *pb.Value_Numeric:
		return s.Numeric
	case *pb.Value_Party:
		return s.Party
	case *pb.Value_Text:
		return s.Text
	case *pb.Value_ContractId:
		return s.ContractId
	case *pb.Value_Optional:
		if s.Optional == nil || s.Optional.Value == nil {
			return nil
		}
		return valueToAny(s.Optional.Value)
	case *pb.Value_List:
		var out []any
		if s.List != nil {
			for _, elem := range s.List.Elements {
				out = append(out, valueToAny(elem))
			}
		}
		if out == nil {
			return []any{}
		}
		return out
	case *pb.Value_TextMap:
		out := map[string]any{}
		if s.TextMap != nil {
			for _, entry := range s.TextMap.Entries {
				out[entry.Key] = valueToAny(entry.Value)
			}
		}
		return out
	case *pb.Value_Record:
		return recordToMap(s.Record)
	case *pb.Value_Variant:
		return map[string]any{
			"constructor": s.Variant.GetConstructor(),
			"value":       valueToAny(s.Variant.GetValue()),
		}
	case *pb.Value_Enum:
		return s.Enum.GetConstructor()
	default:
		return nil
	}
}

func recordToMap(r *pb.Record) map[string]any {
	out := map[string]any{}
	if r == nil {
		return out
	}
	for _, field := range r.Fields {
		out[field.Label] = valueToAny(field.Value)
	}
	return out
}

func decodeDamlDate(days int32) string {
	epoch := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	return epoch.AddDate(0, 0, int(days)).Format("2006-01-02")
}

func decodeDamlTimestamp(micros int64) string {
	secs := micros / 1_000_000
	nanos := (micros % 1_000_000) * int64(time.Microsecond)
	return time.Unix(secs, nanos).UTC().Format(time.RFC3339Nano)
}

func numericToFloat(v any) float64 {
	switch t := v.(type) {
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	case float64:
		return t
	case int64:
		return float64(t)
	default:
		return 0
	}
}
