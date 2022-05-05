package auth

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotktrix/internal/gotktrix"
	"github.com/diamondburned/gotktrix/internal/secret"
	"github.com/diamondburned/gotrix/matrix"
	"github.com/pkg/errors"
)

type Account struct {
	Server    string `json:"server"`
	Token     string `json:"token"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

func copyAccount(client *gotktrix.Client) (*Account, error) {
	id, err := client.Whoami()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get whoami")
	}

	username, _, err := id.Parse()
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse user ID")
	}

	var avatarURL string
	if mxc, _ := client.AvatarURL(client.UserID); mxc != nil {
		avatarURL, _ = client.SquareThumbnail(*mxc, avatarSize, gtkutil.ScaleFactor())
	}

	return &Account{
		Server:    client.HomeServerScheme + "://" + client.HomeServer,
		Token:     client.AccessToken,
		UserID:    string(client.UserID),
		Username:  username,
		AvatarURL: avatarURL,
	}, nil
}

func saveAccount(driver secret.Driver, a *Account) error {
	accIDs, _ := listAccountIDs(driver)

	for _, id := range accIDs {
		if id == matrix.UserID(a.UserID) {
			// Account is already in the list. We only need to override the
			// data.
			goto added
		}
	}

	accIDs = append(accIDs, matrix.UserID(a.UserID))
	if err := saveAccountIDs(driver, accIDs); err != nil {
		return errors.Wrap(err, "failed to save account IDs")
	}

added:
	b, err := json.Marshal(a)
	if err != nil {
		return errors.Wrap(err, "failed to marshal account")
	}

	if err := driver.Set("account:"+a.UserID, b); err != nil {
		return errors.Wrap(err, "failed to set account secret")
	}

	return nil
}

func saveAccountIDs(driver secret.Driver, ids []matrix.UserID) error {
	b, err := json.Marshal(ids)
	if err != nil {
		return errors.Wrap(err, "failed to marshal")
	}

	if err := driver.Set("accounts", b); err != nil {
		return errors.Wrap(err, "failed to set secret")
	}

	return nil
}

func loadAccounts(ctx context.Context, driver secret.Driver) ([]Account, error) {
	accIDs, err := listAccountIDs(driver)
	if err != nil || len(accIDs) == 0 {
		return nil, err
	}

	var accounts []Account
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

		var acc Account
		if err := json.Unmarshal(b, &acc); err != nil {
			errs = append(errs, errors.Wrap(err, "failed to decode account JSON"))
			continue
		}

		accounts = append(accounts, acc)
	}

	if len(errs) == 0 {
		return accounts, nil
	}

	errMsg := strings.Builder{}
	errMsg.WriteString(locale.Sprintf(ctx, "Encountered %d error(s):", len(errs)))
	errMsg.WriteByte('\n')

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
