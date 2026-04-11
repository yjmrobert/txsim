package provider

import "encoding/json"

func jsonCompact(v any) ([]byte, error) { return json.Marshal(v) }

func jsonIndent(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }
