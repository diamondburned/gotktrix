package gotktrix

import (
	"context"

	"github.com/diamondburned/gotrix"
	"github.com/diamondburned/gotrix/api"
	"github.com/diamondburned/gotrix/matrix"
)

// ClientAuth holds a partial client.
type ClientAuth struct {
	c *gotrix.Client
	o Opts
}

// Discover wraps around gotrix.DiscoverWithClienT.
func Discover(serverName string, opts Opts) (*ClientAuth, error) {
	opts.init()

	c, err := gotrix.DiscoverWithClient(opts.Client, serverName)
	if err != nil {
		c, err = gotrix.NewWithClient(opts.Client, serverName)
		if err != nil {
			return nil, err
		}
	}

	return &ClientAuth{
		c: c,
		o: opts,
	}, nil
}

// WithContext creates a copy of ClientAuth that uses the provided context.
func (a *ClientAuth) WithContext(ctx context.Context) *ClientAuth {
	return &ClientAuth{
		c: a.c.WithContext(ctx),
		o: a.o,
	}
}

// LoginPassword authenticates the client using the provided username and
// password.
func (a *ClientAuth) LoginPassword(username, password string) (*Client, error) {
	err := a.c.Client.Login(api.LoginArg{
		Type: matrix.LoginPassword,
		Identifier: matrix.Identifier{
			Type: matrix.IdentifierUser,
			User: username,
		},
		Password:                 password,
		InitialDeviceDisplayName: deviceName,
	})
	if err != nil {
		return nil, err
	}
	return wrapClient(a.c, a.o)
}

// LoginToken authenticates the client using the provided token.
func (a *ClientAuth) LoginToken(token string) (*Client, error) {
	err := a.c.Client.Login(api.LoginArg{
		Type:                     matrix.LoginToken,
		Token:                    deviceName,
		InitialDeviceDisplayName: deviceName,
	})
	if err != nil {
		return nil, err
	}
	return wrapClient(a.c, a.o)
}

// LoginSSO returns the HTTP address for logging in as SSO and the channel
// indicating if the user is done or not.
func (a *ClientAuth) LoginSSO(done func(*Client, error)) (string, error) {
	address, wait, err := a.c.LoginSSO()
	if err != nil {
		return "", err
	}

	go func() {
		if err := wait(); err != nil {
			done(nil, err)
			return
		}

		done(wrapClient(a.c, a.o))
	}()

	return address, nil
}

// LoginMethods returns the login methods supported by the homeserver.
func (a *ClientAuth) LoginMethods() ([]matrix.LoginMethod, error) {
	return a.c.Client.GetLoginMethods()
}
