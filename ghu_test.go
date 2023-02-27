package ghu_test

import (
	"bytes"
	"github.com/semirm-dev/ghu"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestReplaceUsername(t *testing.T) {
	currentConfig := `[user]
	name = invaliduser
	[url "git@github.com:"]
		insteadOf = https://github.com/
	[core]
		excludesfile = /Users/semirmahovkic/.gitignore
	[pull]
		rebase = true`

	expectedConfig := `[user]
	name = user-1
	[url "git@github.com:"]
		insteadOf = https://github.com/
	[core]
		excludesfile = /Users/semirmahovkic/.gitignore
	[pull]
		rebase = true
`
	ghConf := bytes.NewBuffer([]byte(currentConfig))

	replacedConfig, err := ghu.ReplaceUsername("user-1", ghConf)
	assert.NoError(t, err)
	assert.Equal(t, expectedConfig, replacedConfig)
}

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

	replacedConfig, err := ghu.ReplaceSSHKey("private", sshConf)
	assert.NoError(t, err)
	assert.Equal(t, expectedConfig, replacedConfig)
}
