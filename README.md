# gotktrix

![screenshot](./.github/screenshot2.png)

Work-in-progress Matrix client in Go and GTK4.

## Theming Notice

Currently, `libadwaita` is enforcing the Adwaita theme onto ALL `libadwaita`
developers, even when people (me included) only want the widget parts of it.

To work around this awful restriction, run with `GTK_THEME=theme-name`.

There may be plans to remove `libadwaita` completely from `gotktrix`. Mobile
support might be slightly worse, but that is a much smaller price to pay than
losing themes.

## What's working?

Basic message sending. Barely usable. Will be usable soon.
