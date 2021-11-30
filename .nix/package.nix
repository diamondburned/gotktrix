{
	src ? ./..,
	lib,
	pkgs,
	buildPkgs ? import ./pkgs.nix {}, # only for overriding
}:

let desktopFile = pkgs.makeDesktopItem {
    desktopName = "gotktrix";
	icon = "gotktrix";
	name = "gotktrix";
	exec = "gotktrix";
	categories = "GTK;GNOME;Chat;Network;";
};

in buildPkgs.buildGoModule {
	inherit src;

	pname = "gotktrix";
	version = "0.0.1-tip";

	# Bump this on go.mod change.
	vendorSha256 = "01x4xz2pypmn2s8rhyahs0hmhpivppmipwm5fx4zh7v094vvm55g";

	buildInputs = with buildPkgs; [
		gtk4
		glib
		graphene
		gdk-pixbuf
		gobjectIntrospection
	];

	nativeBuildInputs = with pkgs; [
		pkgconfig
		wrapGAppsHook
	];

	preFixup = ''
		mkdir -p $out/share/icons/hicolor/256x256/apps/ $out/share/applications/
		# Install the desktop file
		cp "${desktopFile}"/share/applications/* $out/share/applications/
		# Install the icon
		cp "${../.github/logo-256.png}" $out/share/icons/hicolor/256x256/apps/gotktrix.png
	'';
}
