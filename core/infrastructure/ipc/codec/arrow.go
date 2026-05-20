package codec

import (
	"bytes"
	"fmt"
	"math"
	"strconv"

	"github.com/apache/arrow/go/v18/arrow"
	"github.com/apache/arrow/go/v18/arrow/array"
	"github.com/apache/arrow/go/v18/arrow/ipc"
	"github.com/apache/arrow/go/v18/arrow/memory"
)

type ArrowCodec struct{}

func (ArrowCodec) Marshal(v any) ([]byte, error) {
	rows, err := toRows(v)
	if err != nil {
		return nil, fmt.Errorf("arrow: marshal: %w", err)
	}
	if len(rows) == 0 {
		rec, err := emptyRecord()
		if err != nil {
			return nil, err
		}
		defer rec.Release()
		return recordToIPC(rec)
	}

	fields, cols := inferSchema(rows)
	schema := arrow.NewSchema(fields, nil)
	rec, err := buildRecord(schema, cols, len(rows))
	if err != nil {
		return nil, fmt.Errorf("arrow: build record: %w", err)
	}
	defer rec.Release()

	return recordToIPC(rec)
}

func (ArrowCodec) Unmarshal(data []byte, v any) error {
	rec, err := ipcToRecord(data)
	if err != nil {
		return fmt.Errorf("arrow: unmarshal: %w", err)
	}
	defer rec.Release()

	rows := recordToRows(rec)

	out, ok := v.(*[]map[string]any)
	if !ok {
		return fmt.Errorf("arrow: unmarshal target must be *[]map[string]any")
	}
	*out = rows
	return nil
}

func toRows(v any) ([]map[string]any, error) {
	switch val := v.(type) {
	case []map[string]any:
		return val, nil
	case map[string]any:
		return []map[string]any{val}, nil
	default:
		return nil, fmt.Errorf("unsupported type %T", v)
	}
}

func emptyRecord() (arrow.Record, error) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "_empty", Type: arrow.PrimitiveTypes.Int32},
	}, nil)
	pool := memory.NewGoAllocator()
	b := array.NewInt32Builder(pool)
	defer b.Release()
	b.AppendNull()
	arr := b.NewArray()
	defer arr.Release()
	return array.NewRecord(schema, []arrow.Array{arr}, 0), nil
}

func inferSchema(rows []map[string]any) ([]arrow.Field, [][]any) {
	if len(rows) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(rows[0]))
	seen := make(map[string]int)
	for _, row := range rows {
		for k := range row {
			if _, ok := seen[k]; !ok {
				seen[k] = len(keys)
				keys = append(keys, k)
			}
		}
	}

	fields := make([]arrow.Field, len(keys))
	cols := make([][]any, len(keys))
	for i, k := range keys {
		dt := inferType(rows, k)
		fields[i] = arrow.Field{Name: k, Type: dt, Nullable: true}
		cols[i] = make([]any, len(rows))
		for j, row := range rows {
			cols[i][j] = row[k]
		}
	}
	return fields, cols
}

func inferType(rows []map[string]any, key string) arrow.DataType {
	for _, row := range rows {
		v, ok := row[key]
		if !ok || v == nil {
			continue
		}
		switch v.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return arrow.PrimitiveTypes.Int64
		case float32, float64:
			return arrow.PrimitiveTypes.Float64
		case bool:
			return arrow.FixedWidthTypes.Boolean
		case string:
			return arrow.BinaryTypes.String
		case []byte:
			return arrow.BinaryTypes.Binary
		default:
			return arrow.BinaryTypes.String
		}
	}
	return arrow.BinaryTypes.String
}

func buildRecord(schema *arrow.Schema, cols [][]any, numRows int) (arrow.Record, error) {
	pool := memory.NewGoAllocator()
	columns := make([]arrow.Array, len(cols))

	for i, col := range cols {
		dt := schema.Field(i).Type
		arr, err := buildColumn(pool, dt, col, numRows)
		if err != nil {
			return nil, fmt.Errorf("column %q: %w", schema.Field(i).Name, err)
		}
		columns[i] = arr
	}

	return array.NewRecord(schema, columns, int64(numRows)), nil
}

func buildColumn(pool memory.Allocator, dt arrow.DataType, col []any, numRows int) (arrow.Array, error) {
	switch dt.ID() {
	case arrow.INT64:
		b := array.NewInt64Builder(pool)
		defer b.Release()
		b.Resize(numRows)
		for _, v := range col {
			if v == nil {
				b.AppendNull()
			} else {
				b.Append(toInt64(v))
			}
		}
		return b.NewArray(), nil
	case arrow.FLOAT64:
		b := array.NewFloat64Builder(pool)
		defer b.Release()
		b.Resize(numRows)
		for _, v := range col {
			if v == nil {
				b.AppendNull()
			} else {
				b.Append(toFloat64(v))
			}
		}
		return b.NewArray(), nil
	case arrow.BOOL:
		b := array.NewBooleanBuilder(pool)
		defer b.Release()
		b.Resize(numRows)
		for _, v := range col {
			if v == nil {
				b.AppendNull()
			} else {
				b.Append(v.(bool))
			}
		}
		return b.NewArray(), nil
	case arrow.BINARY:
		b := array.NewBinaryBuilder(pool, dt.(*arrow.BinaryType))
		defer b.Release()
		b.Resize(numRows)
		for _, v := range col {
			if v == nil {
				b.AppendNull()
			} else {
				b.Append(v.([]byte))
			}
		}
		return b.NewArray(), nil
	default:
		b := array.NewStringBuilder(pool)
		defer b.Release()
		b.Resize(numRows)
		for _, v := range col {
			if v == nil {
				b.AppendNull()
			} else {
				b.Append(fmt.Sprintf("%v", v))
			}
		}
		return b.NewArray(), nil
	}
}

func toInt64(v any) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int8:
		return int64(val)
	case int16:
		return int64(val)
	case int32:
		return int64(val)
	case int64:
		return val
	case uint:
		return int64(val)
	case uint8:
		return int64(val)
	case uint16:
		return int64(val)
	case uint32:
		return int64(val)
	case uint64:
		return int64(val)
	case float32:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		return n
	default:
		return 0
	}
}

func toFloat64(v any) float64 {
	switch val := v.(type) {
	case float32:
		return float64(val)
	case float64:
		return val
	case int:
		return float64(val)
	case int8:
		return float64(val)
	case int16:
		return float64(val)
	case int32:
		return float64(val)
	case int64:
		return float64(val)
	case uint:
		return float64(val)
	case uint8:
		return float64(val)
	case uint16:
		return float64(val)
	case uint32:
		return float64(val)
	case uint64:
		return float64(val)
	case string:
		n, _ := strconv.ParseFloat(val, 64)
		return n
	default:
		return math.NaN()
	}
}

func recordToIPC(rec arrow.Record) ([]byte, error) {
	var buf bytes.Buffer
	w := ipc.NewWriter(&buf, ipc.WithSchema(rec.Schema()))
	if err := w.Write(rec); err != nil {
		return nil, fmt.Errorf("arrow: ipc write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("arrow: ipc close: %w", err)
	}
	return buf.Bytes(), nil
}

func ipcToRecord(data []byte) (arrow.Record, error) {
	buf := memory.NewBufferBytes(data)
	reader, err := ipc.NewReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("arrow: ipc reader: %w", err)
	}

	if !reader.Next() {
		return nil, fmt.Errorf("arrow: no record in IPC data")
	}
	rec := reader.Record()
	rec.Retain()
	return rec, nil
}

func recordToRows(rec arrow.Record) []map[string]any {
	numRows := int(rec.NumRows())
	if numRows == 0 {
		return nil
	}

	rows := make([]map[string]any, numRows)
	for i := range rows {
		rows[i] = make(map[string]any)
	}

	for j, col := range rec.Columns() {
		name := rec.Schema().Field(j).Name
		for i := 0; i < numRows; i++ {
			if col.IsNull(i) {
				rows[i][name] = nil
			} else {
				rows[i][name] = extractValue(col, i)
			}
		}
	}
	return rows
}

func extractValue(col arrow.Array, i int) any {
	switch arr := col.(type) {
	case *array.Int64:
		return arr.Value(i)
	case *array.Float64:
		return arr.Value(i)
	case *array.Boolean:
		return arr.Value(i)
	case *array.String:
		return arr.Value(i)
	case *array.Binary:
		return arr.Value(i)
	default:
		return fmt.Sprintf("%v", col.GetOneForMarshal(i))
	}
}
