package database_test

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/coterie/internal/database"
)

func TestJSONBValue(t *testing.T) {
	t.Run("empty stores an empty object", func(t *testing.T) {
		v, err := database.JSONB{}.Value()
		if err != nil {
			t.Fatal(err)
		}
		if v != "{}" {
			t.Fatalf("Value() = %v, want {}", v)
		}
	})

	t.Run("valid json stores as text", func(t *testing.T) {
		v, err := database.JSONB(`{"mode":"equal"}`).Value()
		if err != nil {
			t.Fatal(err)
		}
		if v != `{"mode":"equal"}` {
			t.Fatalf("Value() = %v, want the stored text", v)
		}
	})

	t.Run("invalid json is rejected", func(t *testing.T) {
		if _, err := (database.JSONB(`{"mode":`)).Value(); err == nil {
			t.Fatal("expected an error for malformed JSON")
		}
	})
}

func TestJSONBScan(t *testing.T) {
	t.Run("null becomes an empty object", func(t *testing.T) {
		var j database.JSONB
		if err := j.Scan(nil); err != nil {
			t.Fatal(err)
		}
		if string(j) != "{}" {
			t.Fatalf("scan(nil) = %q, want {}", j)
		}
	})

	t.Run("bytes are copied", func(t *testing.T) {
		src := []byte(`{"a":1}`)
		var j database.JSONB
		if err := j.Scan(src); err != nil {
			t.Fatal(err)
		}
		src[2] = 'z'
		if string(j) != `{"a":1}` {
			t.Fatalf("scanned value tracked the source buffer: %q", j)
		}
	})

	t.Run("text is stored", func(t *testing.T) {
		var j database.JSONB
		if err := j.Scan(`{"a":1}`); err != nil {
			t.Fatal(err)
		}
		if string(j) != `{"a":1}` {
			t.Fatalf("scan(text) = %q, want the text", j)
		}
	})

	t.Run("unsupported type is rejected", func(t *testing.T) {
		var j database.JSONB
		if err := j.Scan(7); err == nil {
			t.Fatal("expected an error for an unsupported source type")
		}
	})
}

func TestJSONBMarshal(t *testing.T) {
	t.Run("empty renders as an object", func(t *testing.T) {
		b, err := json.Marshal(database.JSONB{})
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != "{}" {
			t.Fatalf("marshal = %s, want {}", b)
		}
	})

	t.Run("stored bytes render as-is", func(t *testing.T) {
		b, err := json.Marshal(database.JSONB(`[1,2]`))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != `[1,2]` {
			t.Fatalf("marshal = %s, want [1,2]", b)
		}
	})
}

func TestJSONBUnmarshal(t *testing.T) {
	t.Run("valid json is copied", func(t *testing.T) {
		src := []byte(`{"mode":"equal"}`)
		var j database.JSONB
		if err := json.Unmarshal(src, &j); err != nil {
			t.Fatal(err)
		}
		src[2] = 'z'
		if string(j) != `{"mode":"equal"}` {
			t.Fatalf("decoded value tracked the input buffer: %q", j)
		}
	})

	t.Run("malformed json is rejected", func(t *testing.T) {
		var j database.JSONB
		if err := json.Unmarshal([]byte(`{"mode":`), &j); err == nil {
			t.Fatal("expected an error for malformed JSON")
		}
	})
}
