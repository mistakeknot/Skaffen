package main

import (
	"strings"
	"testing"
)

func TestSoundpostValidate(t *testing.T) {
	cases := []struct {
		name    string
		sp      Soundpost
		wantErr bool
	}{
		{
			name: "valid empty",
			sp:   Soundpost{Empty: true},
		},
		{
			name: "valid empty with error",
			sp:   Soundpost{Empty: true, Error: "binary missing"},
		},
		{
			name: "valid full soundpost",
			sp: Soundpost{
				Empty:        false,
				Lens:         "soundpost-luthier",
				Rationale:    "You're naming components but the load-bearing piece is the single-object response.",
				NextQuestion: "If you added a 'list of options' field, what would have to bend?",
			},
		},
		{
			name:    "empty=true but lens set",
			sp:      Soundpost{Empty: true, Lens: "x"},
			wantErr: true,
		},
		{
			name:    "empty=false but missing rationale",
			sp:      Soundpost{Empty: false, Lens: "x", NextQuestion: "q?"},
			wantErr: true,
		},
		{
			name:    "empty=false but missing next_question",
			sp:      Soundpost{Empty: false, Lens: "x", Rationale: "r"},
			wantErr: true,
		},
		{
			name:    "empty=false but missing lens",
			sp:      Soundpost{Empty: false, Rationale: "r", NextQuestion: "q?"},
			wantErr: true,
		},
		{
			name: "lens too long",
			sp: Soundpost{
				Empty: false, Lens: strings.Repeat("x", 129),
				Rationale: "r", NextQuestion: "q?",
			},
			wantErr: true,
		},
		{
			name: "rationale too long",
			sp: Soundpost{
				Empty: false, Lens: "x",
				Rationale: strings.Repeat("y", 801), NextQuestion: "q?",
			},
			wantErr: true,
		},
		{
			name: "next_question too long",
			sp: Soundpost{
				Empty: false, Lens: "x", Rationale: "r",
				NextQuestion: strings.Repeat("z", 401),
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.sp.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("want error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseSoundpost(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr bool
		want    Soundpost
	}{
		{
			name: "clean empty",
			raw:  `{"empty": true}`,
			want: Soundpost{Empty: true},
		},
		{
			name: "clean full",
			raw:  `{"empty": false, "lens": "L", "rationale": "R", "next_question": "Q?"}`,
			want: Soundpost{Empty: false, Lens: "L", Rationale: "R", NextQuestion: "Q?"},
		},
		{
			name: "wrapped in markdown fence",
			raw:  "```json\n{\"empty\": true}\n```",
			want: Soundpost{Empty: true},
		},
		{
			name: "wrapped in prose preamble",
			raw:  "Here is the soundpost: {\"empty\": false, \"lens\": \"L\", \"rationale\": \"R\", \"next_question\": \"Q?\"}",
			want: Soundpost{Empty: false, Lens: "L", Rationale: "R", NextQuestion: "Q?"},
		},
		{
			name:    "no JSON",
			raw:     "I think this is a casual message.",
			wantErr: true,
		},
		{
			name:    "malformed json",
			raw:     `{"empty": true,}`,
			wantErr: true,
		},
		{
			name:    "schema violation (empty=true but lens set)",
			raw:     `{"empty": true, "lens": "L"}`,
			wantErr: true,
		},
		{
			name: "string-with-braces does not confuse parser",
			raw:  `{"empty": false, "lens": "L", "rationale": "R with } in it", "next_question": "Q?"}`,
			want: Soundpost{Empty: false, Lens: "L", Rationale: "R with } in it", NextQuestion: "Q?"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSoundpost(tc.raw)
			if tc.wantErr && err == nil {
				t.Fatalf("want error, got %+v", got)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.wantErr {
				if got != tc.want {
					t.Fatalf("got %+v, want %+v", got, tc.want)
				}
			}
		})
	}
}
