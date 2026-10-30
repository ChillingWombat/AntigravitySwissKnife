package importer

import (
	"reflect"
	"strings"
	"testing"
)

func TestCandidateSessionJSONIsMetadataOnly(t *testing.T) {
	forbidden := map[string]bool{
		"content":    true,
		"transcript": true,
		"messages":   true,
		"prompt":     true,
		"response":   true,
		"body":       true,
	}
	typ := reflect.TypeOf(CandidateSession{})
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if forbidden[strings.ToLower(name)] {
			t.Fatalf("CandidateSession exposes raw conversation field %q", name)
		}
	}
}
