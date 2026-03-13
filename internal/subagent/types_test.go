package subagent

import "testing"

func TestSubagentType_Validate(t *testing.T) {
	tests := []struct {
		name    string
		st      SubagentType
		wantErr bool
	}{
		{
			name:    "valid explore type",
			st:      SubagentType{Name: "explore", Description: "Read-only", Tools: []string{"read", "grep", "glob", "ls"}, ReadOnly: true, MaxTurns: 10},
			wantErr: false,
		},
		{
			name:    "missing name",
			st:      SubagentType{Description: "No name", Tools: []string{"read"}, MaxTurns: 10},
			wantErr: true,
		},
		{
			name:    "zero max turns defaults",
			st:      SubagentType{Name: "test", Description: "Test", Tools: []string{"read"}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.st.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSubagentStatus_String(t *testing.T) {
	if StatusDone.String() != "done" {
		t.Errorf("StatusDone.String() = %q, want 'done'", StatusDone.String())
	}
	if StatusFailed.String() != "failed" {
		t.Errorf("StatusFailed.String() = %q, want 'failed'", StatusFailed.String())
	}
}
