{
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
	pname = "gotktrix";
	version = "0.0.1-tip";

	src = pkgs.fetchFromGitHub {
		owner  = "diamondburned";
		repo   = "gotktrix";
		rev    = "ec6b24643ada60a037de5b8c31064ba5b92ce550";
		sha256 = "1pcd8qaggki3d4innw89nn6gk1rdp7wwv6zq8s5mbrzr55c60ylz";
	};

	# Bump this on go.mod change.
	vendorSha256 = "0qcm426wm9lpf9qy2pg81x6pcaykphpxrxm3a4xpd7ll10zi3fpp";

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
