{
	src ? ./..,
	lib,
	pkgs,
	suffix ? "",
	buildPkgs ? import ./pkgs.nix {}, # only for overriding
	goPkgs    ? buildPkgs,
	wrapGApps ? true,
}:

let desktopFile = pkgs.makeDesktopItem {
    desktopName = "gotktrix";
	icon = "gotktrix";
	name = "gotktrix";
	exec = "gotktrix";
	categories = "GTK;GNOME;Chat;Network;";
};

in goPkgs.buildGoModule {
	inherit src;

	pname = "gotktrix" + suffix;
	version = "0.0.1-tip";

	# Bump this on go.mod change.
	vendorSha256 = "0qzrljq8cdhl8jfpsdami68zlf2bsbjwbj1n2jamr3fd9y679n97";

	buildInputs = with buildPkgs; [
		gtk4
		glib
		graphene
		gdk-pixbuf
		gobjectIntrospection
	];

	nativeBuildInputs = with pkgs; [
		pkgconfig
	] ++ (lib.optional wrapGApps [ pkgs.wrapGAppsHook ]);

	subPackages = [ "." ];

	preFixup = ''
		mkdir -p $out/share/icons/hicolor/256x256/apps/ $out/share/applications/
		# Install the desktop file
		cp "${desktopFile}"/share/applications/* $out/share/applications/
		# Install the icon
		cp "${../.github/logo-256.png}" $out/share/icons/hicolor/256x256/apps/gotktrix.png
	'';
}
