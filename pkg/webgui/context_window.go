package webgui

import (
	"database/sql"
	"fmt"
	"path/filepath"
)

// hostContextWindow is the active context budget reported by the Antigravity
// language server in ChatStartMetadata.context_window_metadata.
type hostContextWindow struct {
	UsedTokens int64
	MaxTokens  int64
}

// readHostContextWindow reads the newest gen_metadata row that carries the
// language server's context window metadata. The conversation DB is the source
// of truth; model labels and provider catalogs are not.
func readHostContextWindow(conversationsDir, convID string) (hostContextWindow, bool) {
	dbPath := filepath.Join(conversationsDir, convID+".db")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro&_busy_timeout=2000", dbPath))
	if err != nil {
		return hostContextWindow{}, false
	}
	defer db.Close()

	rows, err := db.Query("SELECT data FROM gen_metadata ORDER BY idx DESC")
	if err != nil {
		return hostContextWindow{}, false
	}
	defer rows.Close()

	for rows.Next() {
		var blob []byte
		if err := rows.Scan(&blob); err != nil {
			continue
		}
		if cw, ok := parseHostContextWindowMetadata(blob); ok {
			return cw, true
		}
	}
	return hostContextWindow{}, false
}

type protoWireField struct {
	number   int
	wireType int
	varint   uint64
	data     []byte
}

func parseProtoWireFields(data []byte) ([]protoWireField, bool) {
	fields := make([]protoWireField, 0, 8)
	for offset := 0; offset < len(data); {
		tag, ok := readProtoVarint(data, &offset)
		if !ok {
			return nil, false
		}
		field := protoWireField{number: int(tag >> 3), wireType: int(tag & 7)}
		if field.number == 0 {
			return nil, false
		}

		switch field.wireType {
		case 0:
			field.varint, ok = readProtoVarint(data, &offset)
		case 1:
			if offset+8 > len(data) {
				return nil, false
			}
			offset += 8
		case 2:
			var size uint64
			size, ok = readProtoVarint(data, &offset)
			if ok && size <= uint64(len(data)-offset) {
				field.data = data[offset : offset+int(size)]
				offset += int(size)
			} else {
				ok = false
			}
		case 5:
			if offset+4 > len(data) {
				return nil, false
			}
			offset += 4
		default:
			return nil, false
		}
		if !ok {
			return nil, false
		}
		fields = append(fields, field)
	}
	return fields, true
}

func readProtoVarint(data []byte, offset *int) (uint64, bool) {
	var value uint64
	for shift := uint(0); shift < 64 && *offset < len(data); shift += 7 {
		b := data[*offset]
		*offset++
		if b < 0x80 {
			return value | uint64(b)<<shift, true
		}
		value |= uint64(b&0x7f) << shift
	}
	return 0, false
}

func firstProtoBytes(fields []protoWireField, number int) ([]byte, bool) {
	for _, field := range fields {
		if field.number == number && field.wireType == 2 {
			return field.data, true
		}
	}
	return nil, false
}

func firstProtoVarint(fields []protoWireField, number int) (uint64, bool) {
	for _, field := range fields {
		if field.number == number && field.wireType == 0 {
			return field.varint, true
		}
	}
	return 0, false
}

// parseHostContextWindowMetadata walks the known protobuf path:
// CortexStepGeneratorMetadata.chat_model
// -> ChatModelMetadata.chat_start_metadata
// -> ChatStartMetadata.context_window_metadata.
func parseHostContextWindowMetadata(blob []byte) (hostContextWindow, bool) {
	top, ok := parseProtoWireFields(blob)
	if !ok {
		return hostContextWindow{}, false
	}
	chatModel, ok := firstProtoBytes(top, 1)
	if !ok {
		return hostContextWindow{}, false
	}
	chatFields, ok := parseProtoWireFields(chatModel)
	if !ok {
		return hostContextWindow{}, false
	}
	chatStart, ok := firstProtoBytes(chatFields, 9)
	if !ok {
		return hostContextWindow{}, false
	}
	startFields, ok := parseProtoWireFields(chatStart)
	if !ok {
		return hostContextWindow{}, false
	}
	contextWindow, ok := firstProtoBytes(startFields, 10)
	if !ok {
		return hostContextWindow{}, false
	}
	contextFields, ok := parseProtoWireFields(contextWindow)
	if !ok {
		return hostContextWindow{}, false
	}

	used, hasUsed := firstProtoVarint(contextFields, 1)
	maxTokens, hasMax := firstProtoVarint(contextFields, 4)
	if !hasMax || maxTokens < 1024 || maxTokens > 20_000_000 {
		return hostContextWindow{}, false
	}
	if !hasUsed || used > uint64(^uint64(0)>>1) {
		used = 0
	}
	return hostContextWindow{
		UsedTokens: int64(used),
		MaxTokens:  int64(maxTokens),
	}, true
}
