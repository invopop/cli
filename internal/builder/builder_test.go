package builder

import (
	"context"
	"testing"

	"github.com/invopop/invocli/internal/config"
	"github.com/stretchr/testify/assert"
	"gitlab.com/flimzy/testy"
)

func Test_buildEnvelope(t *testing.T) {
	tests := []struct {
		name string
		file string
		err  string
	}{
		{
			name: "file not found",
			file: "non-existant file",
			err:  "reading file: open non-existant file: no such file or directory",
		},
		{
			name: "invalid yaml",
			file: "testdata/invalid.yaml",
			err:  "parsing source YAML: error unmarshaling JSON: json: cannot unmarshal string into Go value of type gobl.Envelope",
		},
		{
			name: "empty head",
			file: "testdata/emptyhead.yaml",
			err:  "invalid/empty header",
		},
		{
			name: "wrong datatype for header",
			file: "testdata/wrongheader.yaml",
			err:  "parsing source YAML: error unmarshaling JSON: json: cannot unmarshal string into Go struct field Envelope.head of type gobl.Header",
		},
		{
			name: "unsupported type",
			file: "testdata/unsupportedtype.yaml",
			err:  `unrecognized envelope type: "oink"`,
		},
		{
			name: "empty doc",
			file: "testdata/emptydoc.yaml",
			err:  "invalid/empty document",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Command{
				conf: &config.Config{
					Region: "ES",
				},
			}
			env, err := c.buildEnvelope(context.Background(), tt.file)
			if tt.err != "" || err != nil {
				assert.EqualError(t, err, tt.err)
			}
			if err != nil {
				return
			}
			if d := testy.DiffInterface(testy.Snapshot(t), env); d != nil {
				t.Error(d)
			}
		})
	}
}
