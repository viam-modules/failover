package common

import (
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"
)

func TestValidate(t *testing.T) {

	tests := []struct {
		name        string
		cfg         *Config
		expectedErr error
	}{
		{
			name: "Valid config should return dependenecies",
			cfg: &Config{
				Primary: "primary",
				Backups: []string{"backup1", "backup2"},
				Timeout: 1,
			},
		},
		{
			name: "If primary is missing, should error",
			cfg: &Config{
				Backups: []string{"backup1", "backup2"},
				Timeout: 1,
			},
			expectedErr: utils.NewConfigValidationFieldRequiredError("path", "primary"),
		},
		{
			name: "If backups are missing, should error",
			cfg: &Config{
				Primary: "primary",
				Timeout: 1,
			},
			expectedErr: utils.NewConfigValidationFieldRequiredError("path", "backups"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps, err := tc.cfg.Validate("path")
			if tc.expectedErr != nil {
				test.That(t, deps, test.ShouldBeNil)
				test.That(t, err.Error(), test.ShouldResemble, tc.expectedErr.Error())
			} else {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, deps, test.ShouldNotBeNil)
			}

		})
	}
}
