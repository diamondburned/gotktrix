{
	src ? ./..,
	lib,
	pkgs,
	internalPkgs ? import ./pkgs.nix {}, # only for overriding
}:

let desktopFile = pkgs.makeDesktopItem {
    desktopName = "gotktrix";
	icon = "gotktrix";
	name = "gotktrix";
	exec = "gotktrix";
	categories = "GTK;GNOME;Chat;Network;";
};

in internalPkgs.buildGoModule {
	inherit src;

	pname = "gotktrix";
	version = "0.0.1-tip";

	# Bump this on go.mod change.
	vendorSha256 = "0jpq44261fllz1fdxyp4sjrlkfrdqcqplm5y95mcjwb3qjdxjb1z";

	buildInputs = with internalPkgs; [
		libadwaita
		gtk4
		glib
		graphene
		gdk-pixbuf
		gobjectIntrospection
	];

	nativeBuildInputs = with pkgs; [ pkgconfig ];

	preFixup = ''
		mkdir -p $out/share/icons/hicolor/256x256/apps/ $out/share/applications/
		# Install the desktop file
		cp "${desktopFile}"/share/applications/* $out/share/applications/
		# Install the icon
		cp "${../.github/logo-256.png}" $out/share/icons/hicolor/256x256/apps/gotktrix.png
	'';

	subPackages = [ "cmd/gotktrix" ];
}
