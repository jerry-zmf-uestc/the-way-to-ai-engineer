package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestLocalAnswerProjectionAndAdaptationStaySeparateFromScoring(t *testing.T) {
	o := Observation{Doc: "timeout", Version: "v1", Text: "30 seconds"}
	request := localAnswerRequest(o)
	data, _ := json.Marshal(request)
	for _, hidden := range []string{"Expected", "Critical", "Split"} {
		if strings.Contains(string(data), hidden) {
			t.Fatal("labels in request", string(data))
		}
	}
	raw := `{"claims":[{"text":"30 seconds","citations":[{"evidence_id":"timeout","version":"v1","quote":"30 seconds"}]}]}`
	a, err := adaptLocalAnswer(raw, o)
	if err != nil || a != (Answer{"30 seconds", "timeout", "v1", false}) {
		t.Fatal(a, err)
	}
	for _, bad := range []string{"```json\n" + raw + "\n```", strings.Replace(raw, "v1", "old", 1), strings.Replace(raw, "30 seconds\"}]", "missing\"}]", 1), `{"claims":[],"abstained":true,"reason":"insufficient_evidence","extra":1}`} {
		if _, err := adaptLocalAnswer(bad, o); !errors.Is(err, ErrRecord) {
			t.Fatal("bad model output adapted", bad, err)
		}
	}
	missing := Observation{Doc: "absent", Missing: "absent"}
	if req := localAnswerRequest(missing); len(req["evidence"].([]map[string]string)) != 0 {
		t.Fatal("missing observation became evidence", req)
	}
	if a, err := adaptLocalAnswer(`{"abstained":true,"reason":"insufficient_evidence","claims":[]}`, missing); err != nil || !a.Abstained {
		t.Fatal(a, err)
	}
}
