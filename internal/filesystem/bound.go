package filesystem

import (
	"github.com/hosting-panel/panel/agent/policy"
)

func AccountPath(username, rel string) (string, error) {
	if rel == "" || rel == "/" {
		return policy.AccountRoot(username), nil
	}
	return policy.WithinAccount(username, policy.AccountRoot(username)+"/"+rel)
}
