package ghu_test

import (
	"bytes"
	"github.com/semirm-dev/ghu"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestProcessSet_GitHubConfig(t *testing.T) {
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

	replacedConfig, _, err := ghu.Set("user-1", "", ghConf, nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedConfig, replacedConfig)
}

func TestProcessSet_SSHConfig(t *testing.T) {
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

	_, replacedConfig, err := ghu.Set("", "private", nil, sshConf)
	assert.NoError(t, err)
	assert.Equal(t, expectedConfig, replacedConfig)
}
