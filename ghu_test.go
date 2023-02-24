package ghu_test

import (
	"github.com/semirm-dev/ghu"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestProcessSet(t *testing.T) {
	testTable := map[string]struct {
		username    string
		sshKey      string
		expectedErr error
	}{
		"valid username and ssh key returns no error": {
			username:    "user-1",
			sshKey:      "ssh-1",
			expectedErr: nil,
		},
	}

	for name, tt := range testTable {
		t.Run(name, func(t *testing.T) {
			err := ghu.ProcessSet(tt.username, tt.sshKey)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}
