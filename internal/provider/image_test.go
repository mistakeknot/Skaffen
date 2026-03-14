package provider

import (
	"encoding/json"
	"testing"
)

func TestContentBlock_ImageJSON(t *testing.T) {
	block := ContentBlock{
		Type: "image",
		Source: &ImageSource{
			Type:      "base64",
			MediaType: "image/png",
			Data:      "iVBORw0KGgo=",
		},
	}
	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	if raw["type"] != "image" {
		t.Errorf("type: got %v, want image", raw["type"])
	}
	if raw["source"] == nil {
		t.Error("source field missing from JSON")
	}
	// text should be omitted
	if _, ok := raw["text"]; ok {
		t.Error("text field should be omitted for image blocks")
	}

	// Round-trip
	var decoded ContentBlock
	json.Unmarshal(data, &decoded)
	if decoded.Source == nil {
		t.Fatal("decoded source is nil")
	}
	if decoded.Source.MediaType != "image/png" {
		t.Errorf("media_type: got %q, want image/png", decoded.Source.MediaType)
	}
	if decoded.Source.Data != "iVBORw0KGgo=" {
		t.Errorf("data: got %q, want iVBORw0KGgo=", decoded.Source.Data)
	}
}

func TestContentBlock_TextJSON_NoSource(t *testing.T) {
	block := ContentBlock{Type: "text", Text: "hello"}
	data, _ := json.Marshal(block)

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	if _, ok := raw["source"]; ok {
		t.Error("source should be omitted for text blocks")
	}
}
