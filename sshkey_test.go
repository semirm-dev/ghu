package ghu_test

import (
	"bytes"
	"github.com/semirm-dev/ghu"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestReplaceSSHKey(t *testing.T) {
	currentConfig := `Host github.com
  AddKeysToAgent yes
  UseKeychain yes
  IdentityFile ~/.ssh/private_git`

	expectedConfig := `Host github.com
  AddKeysToAgent yes
  UseKeychain yes
  IdentityFile ~/.ssh/private
`
	sshConf := bytes.NewBuffer([]byte(currentConfig))

	replacedConfig, err := ghu.ReplaceSSHKey(sshConf, "private", "")
	assert.NoError(t, err)
	assert.Equal(t, expectedConfig, replacedConfig)
}

func TestReplaceSSHKey_MultipleHosts(t *testing.T) {
	currentConfig := `Host github.com
  AddKeysToAgent yes
  UseKeychain yes
  IdentityFile ~/.ssh/private_git

Host github.com/other-host
  AddKeysToAgent yes
  UseKeychain yes
  IdentityFile ~/.ssh/other`

	expectedConfig := `Host github.com
  AddKeysToAgent yes
  UseKeychain yes
  IdentityFile ~/.ssh/private

Host github.com/other-host
  AddKeysToAgent yes
  UseKeychain yes
  IdentityFile ~/.ssh/other
`
	sshConf := bytes.NewBuffer([]byte(currentConfig))

	replacedConfig, err := ghu.ReplaceSSHKey(sshConf, "private", "github.com")
	assert.NoError(t, err)
	assert.Equal(t, expectedConfig, replacedConfig)
}
