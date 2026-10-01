package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"connectrpc.com/vanguard"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Request bodies naming an unknown field are refused (GLD-050): a misspelt
// field (say, a typo inside initialGovernance) would otherwise be dropped in
// silence and the request applied without it. The SPA ships with the server,
// so there is no older or newer client to stay lenient for. Unknown query
// parameters answer 400 too (GLD-043, cmd/goeland-server).

// StrictJSONOption makes a Connect handler refuse JSON bodies with unknown
// fields (INVALID_ARGUMENT). Every module adds it to its handler options.
func StrictJSONOption() connect.Option {
	return connect.WithOptions(
		connect.WithCodec(strictJSONCodec{name: "json"}),
		connect.WithCodec(strictJSONCodec{name: "json; charset=utf-8"}),
	)
}

// NewStrictJSONCodec is the Vanguard codec factory for the REST bindings: the
// default JSON codec (unpopulated fields emitted) without DiscardUnknown.
// Pass it with vanguard.WithCodec to every transcoder.
func NewStrictJSONCodec(res vanguard.TypeResolver) vanguard.Codec {
	codec := vanguard.NewJSONCodec(res)
	codec.UnmarshalOptions.DiscardUnknown = false
	return codec
}

// strictJSONCodec is connect-go's JSON codec without DiscardUnknown.
type strictJSONCodec struct {
	name string
}

// Name returns the codec name ("json" or "json; charset=utf-8").
func (c strictJSONCodec) Name() string { return c.name }

// Marshal encodes a protobuf message as connect-go does.
func (c strictJSONCodec) Marshal(message any) ([]byte, error) {
	msg, ok := message.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("%T is not a protobuf message", message)
	}
	return protojson.MarshalOptions{}.Marshal(msg)
}

// Unmarshal decodes a protobuf message, refusing unknown fields.
func (c strictJSONCodec) Unmarshal(data []byte, message any) error {
	msg, ok := message.(proto.Message)
	if !ok {
		return fmt.Errorf("%T is not a protobuf message", message)
	}
	if len(data) == 0 {
		return errors.New("zero-length payload is not a valid JSON object")
	}
	return protojson.UnmarshalOptions{}.Unmarshal(data, msg)
}

// MarshalStable encodes with compacted whitespace, as connect-go does.
func (c strictJSONCodec) MarshalStable(message any) ([]byte, error) {
	data, err := c.Marshal(message)
	if err != nil {
		return nil, err
	}
	compacted := bytes.NewBuffer(data[:0])
	if err := json.Compact(compacted, data); err != nil {
		return nil, err
	}
	return compacted.Bytes(), nil
}

// IsBinary reports false: JSON is a text format.
func (c strictJSONCodec) IsBinary() bool { return false }
