package common

import (
	"encoding/binary"
	"fmt"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/global/types"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/metadata"
	"github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/utilities"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"strconv"
	"time"
)

func BindMutations(assignments []types.Assignment, usingTimestamp types.DynamicValue, values *types.QueryParameterValues, b *types.BigtableWriteMutation) error {
	for _, assignment := range assignments {
		// skip primary keys because those are part of the row key which is handled separately
		if assignment.Column().IsPrimaryKey {
			continue
		}
		switch v := assignment.(type) {
		case *types.AssignmentCounterIncrement:
			value, err := utilities.GetValueInt64(v.Value(), values)
			if err != nil {
				return err
			}
			if v.Op == types.PLUS {
				// nothing to do
			} else if v.Op == types.MINUS {
				value = value * -1
			} else {
				return fmt.Errorf("unhandled counter operator '%s'", v.Op)
			}
			b.AddMutations(types.NewBigtableCounterOp(v.Column().ColumnFamily, value))
		case *types.ComplexAssignmentSet:
			mutations, err := encodeSetValue(v, values)
			if err != nil {
				return err
			}
			b.AddMutations(mutations...)
		case *types.ComplexAssignmentAppend:
			colType := v.Column().CQLType.Code()
			if colType == types.LIST {
				lt := v.Column().CQLType.(*types.ListType)
				if v.Operator == types.PLUS {
					value, err := utilities.GetValueSlice(v.Value(), values)
					if err != nil {
						return err
					}
					mutations, err := addListElements(value, v.Column().ColumnFamily, lt, v.IsPrepend)
					if err != nil {
						return err
					}
					b.AddMutations(mutations...)
				} else if v.Operator == types.MINUS {
					value, err := utilities.GetValueSlice(v.Value(), values)
					if err != nil {
						return err
					}
					var bigtableValues []types.BigtableValue
					for _, lv := range value {
						bv, err := EncodeScalarForBigtable(lv, lt.ElementType())
						if err != nil {
							return err
						}
						bigtableValues = append(bigtableValues, bv)
					}
					b.AddMutations(types.NewBigtableDeleteListElementsOp(v.Column().ColumnFamily, bigtableValues))
				} else {
					return fmt.Errorf("unsupported append operation on list %s", v.Operator)
				}
			} else if colType == types.SET {
				if v.Operator == types.PLUS {
					value, err := utilities.GetValueSlice(v.Value(), values)
					if err != nil {
						return err
					}
					ops, err := addSetElements(value, v.Column().ColumnFamily)
					if err != nil {
						return err
					}
					b.AddMutations(ops...)
				} else if v.Operator == types.MINUS {
					value, err := utilities.GetValueSlice(v.Value(), values)
					if err != nil {
						return err
					}
					err = removeSetElements(value, v.Column().ColumnFamily, b)
					if err != nil {
						return err
					}
				} else {
					return fmt.Errorf("unsupported append operation on set %s", v.Operator)
				}
			} else if colType == types.MAP {
				mt := v.Column().CQLType.(*types.MapType)
				if v.Operator == types.PLUS {
					value, err := utilities.GetValueMap(v.Value(), values)
					if err != nil {
						return err
					}
					err = addMapEntries(value, mt, assignment.Column(), b)
					if err != nil {
						return err
					}
				} else if v.Operator == types.MINUS {
					value, err := utilities.GetValueSlice(v.Value(), values)
					if err != nil {
						return err
					}
					err = removeMapEntries(value, v.Column(), b)
					if err != nil {
						return err
					}
				} else {
					return fmt.Errorf("unsupported append operation on set %s", v.Operator)
				}
			} else {
				return fmt.Errorf("unhandled add assignment column type: %s", v.Column().CQLType.String())
			}
		case *types.ComplexAssignmentUpdateListIndex:
			value, err := v.Value().GetValue(values)
			if err != nil {
				return err
			}
			lt, ok := v.Column().CQLType.(*types.ListType)
			if !ok {
				return fmt.Errorf("cannot set list value on column type %s", v.Column().CQLType.String())
			}
			encoded, err := EncodeScalarForBigtable(value, lt.ElementType())
			if err != nil {
				return err
			}
			b.AddMutations(types.NewBigtableSetIndexOp(v.Column().ColumnFamily, v.Index, encoded))
		case *types.ComplexAssignmentUpdateMapValue:
			value, err := v.Value().GetValue(values)
			if err != nil {
				return err
			}
			mt, ok := v.Column().CQLType.(*types.MapType)
			if !ok {
				return fmt.Errorf("cannot set map value on column type %s", v.Column().CQLType.String())
			}
			op, err := mapValueToBigtable(v.Key, value, mt, v.Column().ColumnFamily)
			if err != nil {
				return err
			}
			b.AddMutations(op)
		default:
			return fmt.Errorf("unhandled assignment op type %T", v)
		}
	}

	var err error
	b.UsingTimestamp, err = BindUsingTimestamp(usingTimestamp, values)
	if err != nil {
		return err
	}
	return nil
}

func deleteColumn(col *types.Column) types.IBigtableMutationOp {
	if col.CQLType.IsCollection() {
		return types.NewDeleteCellsOp(col.ColumnFamily)
	} else if col.CQLType.Code() == types.COUNTER {
		// Counters are stored with an empty qualifier.
		return types.NewDeleteColumnOp(types.BigtableColumn{Family: col.ColumnFamily, Column: ""})
	} else {
		return types.NewDeleteColumnOp(types.BigtableColumn{Family: col.ColumnFamily, Column: types.ColumnQualifier(col.Name)})
	}
}

func encodeSetValue(assignment *types.ComplexAssignmentSet, values *types.QueryParameterValues) ([]types.IBigtableMutationOp, error) {
	col := assignment.Column()

	var results []types.IBigtableMutationOp
	if col.CQLType.IsCollection() {
		// clear what's already there
		results = append(results, types.NewDeleteCellsOp(col.ColumnFamily))
	}

	value, err := assignment.Value().GetValue(values)
	if err != nil {
		return nil, err
	}
	// clear the column if the value is explicitly set to nil
	if value == nil {
		return []types.IBigtableMutationOp{deleteColumn(col)}, nil
	}

	if col.CQLType.Code() == types.MAP {
		mt := col.CQLType.(*types.MapType)
		mv, err := utilities.GetValueMap(assignment.Value(), values)
		if err != nil {
			return nil, err
		}
		for k, v := range mv {
			keyEncoded, err := scalarToColumnQualifier(k)
			if err != nil {
				return nil, err
			}
			valueBytes, err := EncodeScalarForBigtable(v, mt.ValueType())
			if err != nil {
				return nil, err
			}
			results = append(results, types.NewWriteCellOp(col.ColumnFamily, keyEncoded, valueBytes))
		}
	} else if col.CQLType.Code() == types.LIST {
		lt := col.CQLType.(*types.ListType)
		lv, err := utilities.GetValueSlice(assignment.Value(), values)
		if err != nil {
			return nil, err
		}
		mutations, err := addListElements(lv, col.ColumnFamily, lt, false)
		if err != nil {
			return nil, err
		}
		results = append(results, mutations...)
	} else if col.CQLType.Code() == types.SET {
		lv, err := utilities.GetValueSlice(assignment.Value(), values)
		if err != nil {
			return nil, err
		}
		mutations, err := addSetElements(lv, col.ColumnFamily)
		if err != nil {
			return nil, err
		}
		results = append(results, mutations...)
	} else {
		value, err := assignment.Value().GetValue(values)
		if err != nil {
			return nil, err
		}
		v, err := EncodeScalarForBigtable(value, col.CQLType)
		if err != nil {
			return nil, err
		}
		results = append(results, types.NewWriteCellOp(col.ColumnFamily, types.ColumnQualifier(col.Name), v))
	}
	return results, nil
}

func addListElements(listValues []types.GoValue, cf types.ColumnFamily, lt *types.ListType, isPrepend bool) ([]types.IBigtableMutationOp, error) {
	var results []types.IBigtableMutationOp
	now := time.Now()
	for i, v := range listValues {
		// Calculate encoded timestamp for the list element
		column := getListIndexColumn(now, i, len(listValues), isPrepend)

		// Format the value
		formattedVal, err := EncodeScalarForBigtable(v, lt.ElementType())
		if err != nil {
			return nil, fmt.Errorf("error converting string to %s value: %w", lt.String(), err)
		}
		results = append(results, types.NewWriteCellOp(cf, column, formattedVal))
	}
	return results, nil
}

// getListIndexColumn generates encoded timestamp bytes.
// Creates timestamp byte representation for specific positions with validation.
// Returns error if position is invalid or encoding fails.
func getListIndexColumn(now time.Time, index int, totalLength int, prepend bool) types.ColumnQualifier {
	nowMilli := now.UnixMilli()
	if prepend {
		nowMilli = referenceTime - (nowMilli - referenceTime)
	}

	nanos := maxNanos - int32(totalLength) + int32(index)
	encodedStr := string(encodeTimestampIndex(nowMilli, nanos))
	return types.ColumnQualifier(encodedStr)
}

// encodeTimestampIndex encodes a timestamp value into bytes.
// Converts timestamp values to byte representation with validation.
// Returns error if timestamp format is invalid or encoding fails.
func encodeTimestampIndex(millis int64, nanos int32) []byte {
	buf := make([]byte, 12) // 8 bytes for millis + 4 bytes for nanos
	binary.BigEndian.PutUint64(buf[0:8], uint64(millis))
	binary.BigEndian.PutUint32(buf[8:12], uint32(nanos))
	return buf
}

// addSetElements adds elements to a set column in raw queries.
// Handles element addition with type validation and conversion.
// Returns error if value type doesn't match set element type or conversion fails.
func addSetElements(setValues []types.GoValue, cf types.ColumnFamily) ([]types.IBigtableMutationOp, error) {
	var results []types.IBigtableMutationOp
	for _, v := range setValues {
		col, err := scalarToColumnQualifier(v)
		if err != nil {
			return nil, err
		}
		// todo use correct col value
		results = append(results, types.NewWriteCellOp(cf, col, []byte("")))
	}
	return results, nil
}

func scalarToColumnQualifier(val types.GoValue) (types.ColumnQualifier, error) {
	switch v := val.(type) {
	case string:
		return types.ColumnQualifier(v), nil
	case *string:
		return types.ColumnQualifier(*v), nil
	case int32:
		return types.ColumnQualifier(strconv.Itoa(int(v))), nil
	case *int32:
		return types.ColumnQualifier(strconv.Itoa(int(*v))), nil
	case int:
		return types.ColumnQualifier(strconv.Itoa(v)), nil
	case *int:
		return types.ColumnQualifier(strconv.Itoa(*v)), nil
	case int64:
		return types.ColumnQualifier(strconv.FormatInt(v, 10)), nil
	case *int64:
		return types.ColumnQualifier(strconv.FormatInt(*v, 10)), nil
	case float32:
		return types.ColumnQualifier(strconv.FormatFloat(float64(v), 'f', -1, 32)), nil
	case *float32:
		return types.ColumnQualifier(strconv.FormatFloat(float64(*v), 'f', -1, 32)), nil
	case float64:
		return types.ColumnQualifier(strconv.FormatFloat(v, 'f', -1, 64)), nil
	case *float64:
		return types.ColumnQualifier(strconv.FormatFloat(*v, 'f', -1, 64)), nil
	case bool:
		if v {
			return "1", nil
		} else {
			return "0", nil
		}
	case *bool:
		if *v {
			return "1", nil
		} else {
			return "0", nil
		}
	case *time.Time:
		return types.ColumnQualifier(strconv.FormatInt(v.UnixMilli(), 10)), nil
	case time.Time:
		return types.ColumnQualifier(strconv.FormatInt(v.UnixMilli(), 10)), nil
	default:
		return "", fmt.Errorf("unsupported type: %T", v)
	}
}

// removeSetElements removes elements from a set column in raw queries.
// Processes element removal with validation of element existence.
// Returns error if element type doesn't match set element type.
func removeSetElements(keys []types.GoValue, colFamily types.ColumnFamily, output *types.BigtableWriteMutation) error {
	for _, key := range keys {
		c, err := scalarToColumnQualifier(key)
		if err != nil {
			return err
		}
		output.AddMutations(types.NewDeleteColumnOp(types.BigtableColumn{Family: colFamily, Column: c}))
	}
	return nil
}

// addMapEntries adds key-value pairs to a map column in raw queries.
// Handles type validation and conversion for both keys and positionalValues.
// Returns error if key/value types don't match map types or conversion fails.
func addMapEntries(mapValue map[types.GoValue]types.GoValue, mt *types.MapType, column *types.Column, output *types.BigtableWriteMutation) error {
	for k, v := range mapValue {
		op, err := mapValueToBigtable(k, v, mt, column.ColumnFamily)
		if err != nil {
			return fmt.Errorf("error converting map key: %w", err)
		}
		output.AddMutations(op)
	}
	return nil
}

func mapValueToBigtable(k types.GoValue, v types.GoValue, mt *types.MapType, cf types.ColumnFamily) (types.IBigtableMutationOp, error) {
	col, err := scalarToColumnQualifier(k)
	if err != nil {
		return nil, fmt.Errorf("error converting map key: %w", err)
	}
	valueFormatted, err := EncodeScalarForBigtable(v, mt.ValueType())
	if err != nil {
		return nil, fmt.Errorf("error converting string to %s value: %w", mt.String(), err)
	}
	return types.NewWriteCellOp(cf, col, valueFormatted), nil
}

func removeMapEntries(keys []types.GoValue, column *types.Column, output *types.BigtableWriteMutation) error {
	for _, key := range keys {
		encodedKey, err := scalarToColumnQualifier(key)
		if err != nil {
			return err
		}
		output.AddMutations(types.NewDeleteColumnOp(types.BigtableColumn{
			Family: column.ColumnFamily,
			Column: encodedKey,
		}))
	}
	return nil
}

// BindQueryParams handles collection operations in prepared queries.
// Processes set, list, and map operations.
// Returns error if collection type is invalid or value encoding fails.
func BindQueryParams(params *types.QueryParameters, positionalValues []*primitive.Value, namedValues map[string]*primitive.Value, pv primitive.ProtocolVersion) (*types.QueryParameterValues, error) {
	if len(positionalValues) > 0 && len(namedValues) > 0 {
		return nil, fmt.Errorf("cannot bind both named and positional parameters")
	}
	// determine binding function based on what we get, because clients may still send positional values, even if named values are used, to save on bandwidth
	if len(positionalValues) > 0 {
		return bindPositionalParams(params, positionalValues, pv)
	} else {
		return bindNamedParams(params, namedValues, pv)
	}
}

func bindPositionalParams(params *types.QueryParameters, values []*primitive.Value, pv primitive.ProtocolVersion) (*types.QueryParameterValues, error) {
	if params.Count() != len(values) {
		return nil, fmt.Errorf("expected %d prepared positional values but got %d", params.Count(), len(values))
	}
	result := types.NewQueryParameterValues(params, time.Now())
	for i, param := range params.Ordered() {
		goVal, err := cassandraValueToGoValue(param.Type, values[i], pv)
		if err != nil {
			return nil, err
		}
		err = result.SetValue(param.Key, goVal)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func bindNamedParams(params *types.QueryParameters, values map[string]*primitive.Value, pv primitive.ProtocolVersion) (*types.QueryParameterValues, error) {
	if params.Count() != len(values) {
		return nil, fmt.Errorf("expected %d prepared named values but got %d", params.Count(), len(values))
	}
	result := types.NewQueryParameterValues(params, time.Now())
	for param, value := range values {
		md, err := params.GetMetadata(types.Parameter(param))
		if err != nil {
			return nil, err
		}
		if !md.IsNamed {
			return nil, fmt.Errorf("cannot bind named parameters to positional parameters")
		}
		goVal, err := cassandraValueToGoValue(md.Type, value, pv)
		if err != nil {
			return nil, err
		}
		err = result.SetValue(md.Key, goVal)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func BindSelectColumns(table *schemaMapping.TableSchema, selectedColumns []types.SelectedColumn) ([]types.BoundSelectColumn, error) {
	var boundColumns []types.BoundSelectColumn
	for _, selectedColumn := range selectedColumns {
		var bc types.BoundSelectColumn
		col, err := table.GetColumn(selectedColumn.ColumnName)
		if err != nil {
			return nil, err
		}
		if selectedColumn.ListIndex != -1 {
			bc = types.NewBoundIndexColumn(col, int(selectedColumn.ListIndex))
		} else if selectedColumn.MapKey != "" {
			bc = types.NewBoundKeyColumn(col, selectedColumn.MapKey)
		} else {
			return nil, fmt.Errorf("unhandled select column type found binding")
		}
		boundColumns = append(boundColumns, bc)
	}
	return boundColumns, nil
}

func BindUsingTimestamp(value types.DynamicValue, values *types.QueryParameterValues) (*types.BoundTimestampInfo, error) {
	if value == nil {
		return &types.BoundTimestampInfo{
			// USING TIMESTAMP is in micros
			Timestamp:         values.Time(),
			HasUsingTimestamp: false,
		}, nil
	}

	t, err := utilities.GetValueInt64(value, values)
	if err != nil {
		return nil, err
	}
	return &types.BoundTimestampInfo{
		// USING TIMESTAMP is in micros
		Timestamp:         time.UnixMicro(t),
		HasUsingTimestamp: true,
	}, nil
}
