# gotktrix

![screenshot](./.github/screenshot7.png)

Matrix client in Go and GTK4.

## Features

List taken from the Features section of the
[Clients Matrix](https://matrix.org/clients-matrix/) page.

- [x] Room directory
- [ ] Room tag showing
- [ ] Room tag editing
- [x] Search joined rooms
- [ ] Room user list
- [ ] Display Room Description
- [ ] Edit Room Description
- [x] Highlights
- [x] Push rules
- [x] Send read markers
- [ ] Display read markers
- [ ] Sending Invites
- [ ] Accepting Invites
- [x] Typing Notification (receive-only)
- [ ] E2EE
- [x] Replies
- [x] Attachment uploading
- [x] Attachment downloading
- [ ] Send stickers
- [x] Send formatted messages markdown
- [x] Rich Text Editor for formatted messages
- [x] Display formatted messages
- [x] Redacting
- [x] Multiple Matrix Accounts
- [ ] New user registration
- [ ] VoIP (non-goal)
- [x] Reactions
- [x] Message editing
- [ ] Room upgrades
- [ ] Localizations (untested, outdated)
- [x] SSO Support

Additionally, the client implements:

- Custom and Unicode Emojis
- Autocompletion
- Mobile support (partial)
- Partial [Spaces](https://github.com/matrix-org/matrix-doc/blob/old_master/proposals/1772-groups-as-rooms.md) support

## Installing

```sh
go install -v github.com/diamondburned/gotktrix@latest
```

### Dependencies

See [package-base.nix](.nix/package-base.nix).

Installing is faster with a patched Go compiler; see
[overlay.nix](.nix/overlay.nix).
