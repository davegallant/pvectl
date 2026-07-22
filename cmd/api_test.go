package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"testing"
)

func TestParseAPIData(t *testing.T) {
	tests := []struct {
		name    string
		data    []string
		want    url.Values
		wantErr bool
	}{
		{"empty", nil, url.Values{}, false},
		{"single pair", []string{"vmid=101"}, url.Values{"vmid": {"101"}}, false},
		{"repeated key", []string{"tag=a", "tag=b"}, url.Values{"tag": {"a", "b"}}, false},
		{"value contains equals", []string{"config=key=value"}, url.Values{"config": {"key=value"}}, false},
		{"missing equals", []string{"novalue"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAPIData(tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseAPIData(%v) error = %v, wantErr %v", tt.data, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.Encode() != tt.want.Encode() {
				t.Errorf("parseAPIData(%v) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestPrintRawJSONEmpty(t *testing.T) {
	if err := printRawJSON(nil); err != nil {
		t.Fatalf("printRawJSON(nil) error = %v", err)
	}
}

func TestPrintRawJSONIndentsInPlace(t *testing.T) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	raw := json.RawMessage(`{"data":{"vmid":101}}`)
	if err := printRawJSON(raw); err != nil {
		t.Fatalf("printRawJSON() error = %v", err)
	}
	_ = w.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}

	want := "{\n  \"data\": {\n    \"vmid\": 101\n  }\n}\n"
	if buf.String() != want {
		t.Errorf("printRawJSON() output = %q, want %q", buf.String(), want)
	}
}
