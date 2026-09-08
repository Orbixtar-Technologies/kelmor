package auth

import (
	"github.com/pquerna/otp/totp"
)

func NewTOTP(issuer, account string) (secret, url string, err error) {
	k, err := totp.Generate(totp.GenerateOpts{Issuer: issuer, AccountName: account})
	if err != nil {
		return "", "", err
	}
	return k.Secret(), k.URL(), nil
}

func VerifyTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}
