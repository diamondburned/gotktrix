{
	pkgs, lib,

	gotktrixSrc ? ./..,
	suffix ? "",
	buildPkgs ? import ./pkgs.nix {}, # only for overriding
	goPkgs ? buildPkgs,
	wrapGApps ? true,
	vendorSha256 ? "052cnzxnkk9q6hns8plhjx29sy7759mmg8hiazp5aa0cb00wd1dj",
}:

goPkgs.buildGoModule {
	src = gotktrixSrc;
	inherit vendorSha256;

	pname = "gotktrix" + suffix;
	version = "0.0.1-tip";

	buildInputs = with buildPkgs; [
		gtk4
		glib
		graphene
		gdk-pixbuf
		gobjectIntrospection
		hicolor-icon-theme

		# Optional
		sound-theme-freedesktop
		libcanberra-gtk3
	];

	nativeBuildInputs = with pkgs; [
		pkgconfig
	] ++ (lib.optional wrapGApps [ pkgs.wrapGAppsHook ]);

	subPackages = [ "." ];

	preFixup = ''
		mkdir -p $out/share/icons/hicolor/256x256/apps/ $out/share/applications/
		# Install the desktop file
		cp "${./com.github.diamondburned.gotktrix.desktop}" $out/share/applications/com.github.diamondburned.gotktrix.desktop
		# Install the icon
		cp "${../.github/logo-256.png}" $out/share/icons/hicolor/256x256/apps/gotktrix.png
	'';
}
