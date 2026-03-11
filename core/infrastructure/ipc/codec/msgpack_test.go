package codec_test

import (
	"testing"

	"github.com/ElioNeto/vyx/core/infrastructure/ipc/codec"
)

type samplePayload struct {
	Route  string `msgpack:"route"`
	Method string `msgpack:"method"`
	UserID int    `msgpack:"user_id"`
}

func TestMsgPackCodec_RoundTrip(t *testing.T) {
	c := codec.MsgPackCodec{}

	want := samplePayload{Route: "/api/users", Method: "POST", UserID: 42}

	data, err := c.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got samplePayload
	if err := c.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got != want {
		t.Errorf("want %+v, got %+v", want, got)
	}
}

func TestMsgPackCodec_EmptyPayload(t *testing.T) {
	c := codec.MsgPackCodec{}

	data, err := c.Marshal(nil)
	if err != nil {
		t.Fatalf("Marshal(nil) error = %v", err)
	}

	var got any
	if err := c.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
}
