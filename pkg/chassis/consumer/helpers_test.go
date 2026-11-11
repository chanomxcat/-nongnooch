//go:build integration

package consumer_test

import "encoding/json"

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
