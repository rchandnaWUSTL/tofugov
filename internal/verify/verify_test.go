package verify

import "testing"

func f(v float64) *float64 { return &v }

func TestParse(t *testing.T) {
	cases := []struct {
		name    string
		in      rawScore
		wantP   float64
		wantB   Band
		wantErr bool
	}{
		{"low", rawScore{PBad: f(0.02)}, 0.02, Low, false},
		{"medium boundary", rawScore{PBad: f(0.15)}, 0.15, Medium, false},
		{"high boundary", rawScore{PBad: f(0.5)}, 0.5, High, false},
		{"clamped above", rawScore{PBad: f(7)}, 1, High, false},
		{"clamped below", rawScore{PBad: f(-0.2)}, 0, Low, false},
		{"missing", rawScore{}, 0, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := parse(c.in, DefaultThresholds)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v", err)
			}
			if !c.wantErr && (s.PBad != c.wantP || s.Band != c.wantB) {
				t.Errorf("got %v/%v, want %v/%v", s.PBad, s.Band, c.wantP, c.wantB)
			}
		})
	}
}

func TestThresholdsValidate(t *testing.T) {
	if err := DefaultThresholds.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (Thresholds{Medium: 0.6, High: 0.5}).Validate(); err == nil {
		t.Fatal("expected error for inverted thresholds")
	}
}
