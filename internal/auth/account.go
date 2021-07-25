package auth

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chanbakjsd/gotrix/matrix"
	"github.com/diamondburned/gotktrix/internal/auth/secret"
	"github.com/pkg/errors"
)

type account struct {
	Server    string `json:"server"`
	Token     string `json:"token"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

func loadAccounts(driver secret.Driver) ([]account, error) {
	accIDs, err := listAccountIDs(driver)
	if err != nil || len(accIDs) == 0 {
		return nil, err
	}

	var accounts []account
	var errs []error

	for _, id := range accIDs {
		b, err := driver.Get("account:" + string(id))
		if err != nil {
			if errors.Is(err, secret.ErrNotFound) {
				// Ignore.
				continue
			}
			errs = append(errs, err)
			continue
		}

		var acc account
		if err := json.Unmarshal(b, &acc); err != nil {
			errs = append(errs, errors.Wrap(err, "failed to decode account JSON"))
			continue
		}

		accounts = append(accounts, acc)
	}

	if len(errs) == 0 {
		return accounts, nil
	}

	var errMsg strings.Builder
	if len(errs) == 1 {
		errMsg.WriteString("Encountered 1 error:\n")
	} else {
		errMsg.WriteString(fmt.Sprintf("Encountered %d errors:\n", len(errs)))
	}

	for _, err := range errs {
		errMsg.WriteString("• ")
		errMsg.WriteString(err.Error())
		errMsg.WriteByte('\n')
	}

	return accounts, errors.New(strings.TrimSuffix(errMsg.String(), "\n"))
}

func listAccountIDs(driver secret.Driver) ([]matrix.UserID, error) {
	b, err := driver.Get("accounts")
	if err != nil {
		if errors.Is(err, secret.ErrNotFound) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to read accounts file")
	}

	var ids []matrix.UserID
	if err := json.Unmarshal(b, &ids); err != nil {
		return nil, errors.Wrap(err, "failed to decode []UserID JSON")
	}

	return ids, nil
}
